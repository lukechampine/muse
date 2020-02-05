package muse

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"gitlab.com/NebulousLabs/Sia/modules"
	"lukechampine.com/frand"
	"lukechampine.com/shard"
	"lukechampine.com/us/ed25519"
	"lukechampine.com/us/hostdb"
	"lukechampine.com/us/renter"
	"lukechampine.com/us/renter/proto"
)

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	// encode nil slices as [] instead of null
	if val := reflect.ValueOf(v); val.Kind() == reflect.Slice && val.Len() == 0 {
		w.Write([]byte("[]\n"))
		return
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "\t")
	enc.Encode(v)
}

type server struct {
	contracts []Contract
	hostSets  map[string][]hostdb.HostPublicKey
	dir       string

	wallet proto.Wallet
	tpool  proto.TransactionPool
	shard  *shard.Client
	mu     sync.Mutex
}

func (s *server) saveContract(c Contract) error {
	path := filepath.Join(s.dir, fmt.Sprintf("%s-%x.contract", c.HostKey.ShortKey(), c.ID[:4]))
	js, _ := json.MarshalIndent(c, "", "  ")
	return ioutil.WriteFile(path, js, 0660)
}

func (s *server) handleContracts(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	var contracts responseContracts
	if setName := req.FormValue("hostset"); setName != "" {
		s.mu.Lock()
		hostKeys, ok := s.hostSets[setName]
		if !ok {
			s.mu.Unlock()
			http.Error(w, "No record of that host set", http.StatusBadRequest)
			return
		}
		// take most recent contract for each host
		set := make(map[hostdb.HostPublicKey]Contract)
		for _, hostKey := range hostKeys {
			set[hostKey] = Contract{}
		}
		for _, c := range s.contracts {
			if d, ok := set[c.HostKey]; ok && c.EndHeight > d.EndHeight {
				set[c.HostKey] = c
			}
		}
		contracts = make([]Contract, 0, len(set))
		for _, c := range set {
			if c.RenterKey != nil {
				contracts = append(contracts, c)
			}
		}
		s.mu.Unlock()
	} else {
		s.mu.Lock()
		contracts = append([]Contract(nil), s.contracts...)
		s.mu.Unlock()
	}

	// fill in addresses
	for i := range contracts {
		addr, err := s.shard.ResolveHostKey(contracts[i].HostKey)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		contracts[i].HostAddress = addr
	}
	writeJSON(w, contracts)
}

func (s *server) handleForm(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	var rf RequestForm
	if err := json.NewDecoder(req.Body).Decode(&rf); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	hostAddr, err := s.shard.ResolveHostKey(rf.HostKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	rf.Settings.NetAddress = hostAddr
	host := hostdb.ScannedHost{
		HostSettings: rf.Settings,
		PublicKey:    rf.HostKey,
	}
	key := ed25519.NewKeyFromSeed(frand.Bytes(32))
	rev, txnSet, err := proto.FormContract(s.wallet, s.tpool, key, host, rf.Funds, rf.StartHeight, rf.EndHeight)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// submit txnSet to tpool
	//
	// NOTE: if the tpool rejects the transaction, we log the error, but
	// still treat the contract as valid. The host has signed the
	// transaction, so presumably they already submitted it to their own
	// tpool without error, and intend to honor the contract. Our tpool
	// *shouldn't* reject the transaction, but it might if we desync from
	// the network somehow.
	submitErr := s.tpool.AcceptTransactionSet(txnSet)
	if submitErr != nil && submitErr != modules.ErrDuplicateTransactionSet {
		log.Println("WARN: contract transaction was not accepted", submitErr)
	}

	c := Contract{
		Contract: renter.Contract{
			HostKey:   rev.HostKey(),
			ID:        rev.ID(),
			RenterKey: key,
		},
		HostAddress: host.NetAddress,
		EndHeight:   rf.EndHeight,
	}
	s.mu.Lock()
	s.contracts = append(s.contracts, c)
	s.mu.Unlock()
	err = s.saveContract(c)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, c)
}

func (s *server) handleRenew(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	var rf RequestRenew
	if err := json.NewDecoder(req.Body).Decode(&rf); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var old Contract
	s.mu.Lock()
	for _, old = range s.contracts {
		if old.ID == rf.ID {
			break
		}
	}
	s.mu.Unlock()
	if old.ID != rf.ID {
		http.Error(w, "no record of that contract", http.StatusBadRequest)
		return
	}

	hostAddr, err := s.shard.ResolveHostKey(old.HostKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	rf.Settings.NetAddress = hostAddr
	host := hostdb.ScannedHost{
		PublicKey:    old.HostKey,
		HostSettings: rf.Settings,
	}
	rev, txnSet, err := proto.RenewContract(s.wallet, s.tpool, old.ID, old.RenterKey, host, rf.Funds, rf.StartHeight, rf.EndHeight)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// submit txnSet to tpool (see handleForm)
	submitErr := s.tpool.AcceptTransactionSet(txnSet)
	if submitErr != nil && submitErr != modules.ErrDuplicateTransactionSet {
		log.Println("WARN: contract transaction was not accepted", submitErr)
	}

	c := Contract{
		Contract: renter.Contract{
			HostKey:   rev.HostKey(),
			ID:        rev.ID(),
			RenterKey: old.RenterKey,
		},
		HostAddress: rf.Settings.NetAddress,
		EndHeight:   rf.EndHeight,
	}
	s.mu.Lock()
	s.contracts = append(s.contracts, c)
	s.mu.Unlock()
	err = s.saveContract(c)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, c)
}

func (s *server) handleHostSets(w http.ResponseWriter, req *http.Request) {
	setName := strings.TrimPrefix(req.URL.Path, "/hostsets/")
	if strings.Contains(setName, "/") {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	switch req.Method {
	case http.MethodGet:
		if setName == "" {
			s.mu.Lock()
			setNames := make([]string, 0, len(s.hostSets))
			for name := range s.hostSets {
				setNames = append(setNames, name)
			}
			s.mu.Unlock()
			sort.Strings(setNames)
			writeJSON(w, setNames)
		} else {
			if setName == "" {
				http.Error(w, "No host set name provided", http.StatusBadRequest)
				return
			}
			s.mu.Lock()
			set, ok := s.hostSets[setName]
			hostKeys := append([]hostdb.HostPublicKey(nil), set...)
			s.mu.Unlock()
			if !ok {
				http.Error(w, "No record of that host set", http.StatusBadRequest)
				return
			}
			writeJSON(w, hostKeys)
		}

	case http.MethodPut:
		if setName == "" {
			http.Error(w, "No host set name provided", http.StatusBadRequest)
			return
		}

		var hostKeys []hostdb.HostPublicKey
		if err := json.NewDecoder(req.Body).Decode(&hostKeys); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		sort.Slice(hostKeys, func(i, j int) bool {
			return hostKeys[i] < hostKeys[j]
		})
		s.mu.Lock()
		s.hostSets[setName] = hostKeys
		if len(hostKeys) == 0 {
			delete(s.hostSets, setName)
		}
		hostSetsJSON, _ := json.MarshalIndent(s.hostSets, "", "  ")
		s.mu.Unlock()
		err := ioutil.WriteFile(filepath.Join(s.dir, "hostSets.json"), hostSetsJSON, 0660)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}

func (s *server) handleScan(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	var rs RequestScan
	if err := json.NewDecoder(req.Body).Decode(&rs); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	hostAddr, err := s.shard.ResolveHostKey(rs.HostKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	host, err := hostdb.Scan(ctx, hostAddr, rs.HostKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, host.HostSettings)
}

// NewServer returns an HTTP handler that serves the muse API.
func NewServer(dir string, wallet proto.Wallet, tpool proto.TransactionPool, shardAddr string) (http.Handler, error) {
	srv := &server{
		wallet: wallet,
		tpool:  tpool,
		shard:  shard.NewClient(shardAddr),
		dir:    dir,
	}

	// load host sets
	hostSetsJSON, err := ioutil.ReadFile(filepath.Join(dir, "hostSets.json"))
	if os.IsNotExist(err) {
		srv.hostSets = make(map[string][]hostdb.HostPublicKey)
	} else {
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(hostSetsJSON, &srv.hostSets); err != nil {
			return nil, err
		}
	}

	// load all contracts
	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, info := range infos {
		if filepath.Ext(info.Name()) != ".contract" {
			continue
		}
		js, err := ioutil.ReadFile(filepath.Join(dir, info.Name()))
		if err != nil {
			return nil, err
		}
		var c Contract
		if err := json.Unmarshal(js, &c); err != nil {
			return nil, err
		}
		srv.contracts = append(srv.contracts, c)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/contracts", srv.handleContracts)
	mux.HandleFunc("/form", srv.handleForm)
	mux.HandleFunc("/renew", srv.handleRenew)
	mux.HandleFunc("/hostsets/", srv.handleHostSets)
	mux.HandleFunc("/scan", srv.handleScan)

	// shard proxy
	shardURL, err := url.Parse(shardAddr)
	if err != nil {
		return nil, err
	}
	mux.Handle("/shard/", &httputil.ReverseProxy{Director: func(req *http.Request) {
		req.URL.Scheme = shardURL.Scheme
		req.URL.Host = shardURL.Host
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/shard")
	}})
	return mux, nil
}
