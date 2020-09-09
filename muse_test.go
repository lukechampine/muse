package muse

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"testing"

	"github.com/pkg/errors"
	"lukechampine.com/frand"
	"lukechampine.com/shard"
	"lukechampine.com/us/ed25519hash"
	"lukechampine.com/us/hostdb"
	"lukechampine.com/us/renterhost"

	"gitlab.com/NebulousLabs/Sia/crypto"
	"gitlab.com/NebulousLabs/Sia/encoding"
	"gitlab.com/NebulousLabs/Sia/modules"
	"gitlab.com/NebulousLabs/Sia/types"
)

type mockCS struct{}

func (mockCS) ConsensusSetSubscribe(s modules.ConsensusSetSubscriber, ccid modules.ConsensusChangeID, cancel <-chan struct{}) error {
	return nil
}

func (mockCS) Synced() bool { return true }

type memPersist struct {
	shard.PersistData
}

func (p *memPersist) Save(data shard.PersistData) error {
	p.PersistData = data
	return nil
}

func (p *memPersist) Load(data *shard.PersistData) error {
	*data = p.PersistData
	return nil
}

type stubWallet struct{}

func (stubWallet) NewWalletAddress() (uh types.UnlockHash, err error)                       { return }
func (stubWallet) SignTransaction(*types.Transaction, []crypto.Hash) (err error)            { return }
func (stubWallet) UnspentOutputs(bool) (us []modules.UnspentOutput, err error)              { return }
func (stubWallet) UnconfirmedParents(types.Transaction) (ps []types.Transaction, err error) { return }
func (stubWallet) UnlockConditions(types.UnlockHash) (uc types.UnlockConditions, err error) { return }

type stubTpool struct{}

func (stubTpool) AcceptTransactionSet([]types.Transaction) (err error) { return }
func (stubTpool) FeeEstimate() (min, max types.Currency, err error)    { return }

func startSHARD(hpk hostdb.HostPublicKey, ann []byte) (string, func() error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return "", nil
	}
	p := &memPersist{
		PersistData: shard.PersistData{
			Hosts: map[hostdb.HostPublicKey][]byte{hpk: ann},
		},
	}
	r, err := shard.NewRelay(mockCS{}, p)
	if err != nil {
		return "", nil
	}
	srv := shard.NewServer(r)
	go http.Serve(l, srv)
	return "http://" + l.Addr().String(), l.Close
}

func TestServer(t *testing.T) {
	// create a host
	host, err := newHost(":0")
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	// create a shard server
	shardAddr, stop := startSHARD(host.PublicKey(), host.announcement())
	defer stop()

	// create the muse server
	dir, _ := ioutil.TempDir("", t.Name())
	defer os.RemoveAll(dir)
	srv, err := NewServer(dir, stubWallet{}, stubTpool{}, shardAddr)
	if err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	go http.Serve(l, srv)

	// test contract formation
	c := NewClient("http://" + l.Addr().String())
	if contracts, err := c.AllContracts(); err != nil {
		t.Fatal(err)
	} else if len(contracts) != 0 {
		t.Fatal(err)
	}

	currentHeight, err := c.SHARD().ChainHeight()
	if err != nil {
		t.Fatal(err)
	}

	settings, err := c.Scan(host.PublicKey())
	if err != nil {
		t.Fatal(err)
	}

	contract, err := c.Form(host.PublicKey(), types.ZeroCurrency, currentHeight, currentHeight+1, settings)
	if err != nil {
		t.Fatal(err)
	}
	if contracts, err := c.AllContracts(); err != nil {
		t.Fatal(err)
	} else if len(contracts) != 1 || contracts[0].ID != contract.ID {
		t.Fatal("wrong contracts:", contracts)
	}
	// test contract renewal
	newContract, err := c.Renew(contract.ID, types.ZeroCurrency, currentHeight, currentHeight+2, settings)
	if err != nil {
		t.Fatal(err)
	}
	if contracts, err := c.AllContracts(); err != nil {
		t.Fatal(err)
	} else if len(contracts) != 2 || contracts[1].ID != newContract.ID {
		t.Fatal("wrong contracts:", contracts)
	}

	// test host sets
	if err = c.SetHostSet("foo", []hostdb.HostPublicKey{host.PublicKey()}); err != nil {
		t.Fatal(err)
	}
	if sets, err := c.HostSets(); err != nil {
		t.Fatal(err)
	} else if len(sets) != 1 || sets[0] != "foo" {
		t.Fatal("wrong host sets:", sets)
	}
	if set, err := c.HostSet("foo"); err != nil {
		t.Fatal(err)
	} else if len(set) != 1 || set[0] != host.PublicKey() {
		t.Fatal("wrong host set:", set)
	}
	if contracts, err := c.Contracts("foo"); err != nil {
		t.Fatal(err)
	} else if len(contracts) != 1 || contracts[0].ID != newContract.ID {
		t.Fatal("wrong contracts:", contracts)
	}

	// test context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := c.WithContext(ctx).AllContracts(); err != nil {
		t.Fatal("should not fail with uncancelled context:", err)
	}
	cancel()
	if _, err := c.WithContext(ctx).AllContracts(); err == nil {
		t.Fatal("should fail with cancelled context")
	}
}

// minimal host, copied from us/ghost

///

type hostContract struct {
	rev  types.FileContractRevision
	sigs [2]types.TransactionSignature
}

type Host struct {
	addr      modules.NetAddress
	secretKey ed25519.PrivateKey
	listener  net.Listener
	contracts map[types.FileContractID]*hostContract
}

func (h *Host) PublicKey() hostdb.HostPublicKey {
	return hostdb.HostKeyFromPublicKey(ed25519hash.ExtractPublicKey(h.secretKey))
}

func (h *Host) settings() hostdb.HostSettings {
	return hostdb.HostSettings{
		NetAddress:         h.addr,
		AcceptingContracts: true,
		WindowSize:         144,
	}
}

func (h *Host) announcement() []byte {
	ann := encoding.Marshal(modules.HostAnnouncement{
		Specifier:  modules.PrefixHostAnnouncement,
		NetAddress: modules.NetAddress(h.listener.Addr().String()),
		PublicKey:  h.PublicKey().SiaPublicKey(),
	})
	sig := ed25519hash.Sign(h.secretKey, crypto.HashBytes(ann))
	return append(ann, sig[:]...)
}

func (h *Host) listen() error {
	for {
		conn, err := h.listener.Accept()
		if err != nil {
			return err
		}
		go h.handleConn(conn)
	}
}

func (h *Host) Close() error {
	return h.listener.Close()
}

func newHost(addr string) (*Host, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	h := &Host{
		addr:      modules.NetAddress(l.Addr().String()),
		listener:  l,
		secretKey: ed25519.NewKeyFromSeed(frand.Bytes(ed25519.SeedSize)),
		contracts: make(map[types.FileContractID]*hostContract),
	}
	go h.listen()
	return h, nil
}

type hostSession struct {
	sess     *renterhost.Session
	contract *hostContract
}

func (h *Host) handleConn(conn net.Conn) error {
	defer conn.Close()
	hs, err := renterhost.NewHostSession(conn, h.secretKey)
	if err != nil {
		return err
	}
	s := &hostSession{sess: hs}
	rpcs := map[renterhost.Specifier]func(*hostSession) error{
		renterhost.RPCSettingsID:           h.rpcSettings,
		renterhost.RPCFormContractID:       h.rpcFormContract,
		renterhost.RPCLockID:               h.rpcLock,
		renterhost.RPCRenewClearContractID: h.rpcRenewContract,
	}
	for {
		id, err := s.sess.ReadID()
		if errors.Cause(err) == renterhost.ErrRenterClosed {
			return nil
		}
		rpcs[id](s)
	}
}

func (h *Host) rpcSettings(s *hostSession) error {
	settings, _ := json.Marshal(h.settings())
	return s.sess.WriteResponse(&renterhost.RPCSettingsResponse{Settings: settings}, nil)
}

func (h *Host) rpcFormContract(s *hostSession) error {
	var req renterhost.RPCFormContractRequest
	s.sess.ReadRequest(&req, 4096)
	txn := req.Transactions[len(req.Transactions)-1]
	fc := txn.FileContracts[0]
	resp := &renterhost.RPCFormContractAdditions{
		Parents: nil,
		Inputs:  nil,
		Outputs: nil,
	}
	s.sess.WriteResponse(resp, nil)
	initRevision := types.FileContractRevision{
		ParentID: txn.FileContractID(0),
		UnlockConditions: types.UnlockConditions{
			PublicKeys: []types.SiaPublicKey{
				req.RenterKey,
				h.PublicKey().SiaPublicKey(),
			},
			SignaturesRequired: 2,
		},
		NewRevisionNumber: 1,

		NewFileSize:           fc.FileSize,
		NewFileMerkleRoot:     fc.FileMerkleRoot,
		NewWindowStart:        fc.WindowStart,
		NewWindowEnd:          fc.WindowEnd,
		NewValidProofOutputs:  fc.ValidProofOutputs,
		NewMissedProofOutputs: fc.MissedProofOutputs,
		NewUnlockHash:         fc.UnlockHash,
	}
	hostRevisionSig := types.TransactionSignature{
		ParentID:       crypto.Hash(initRevision.ParentID),
		CoveredFields:  types.CoveredFields{FileContractRevisions: []uint64{0}},
		PublicKeyIndex: 1,
		Signature:      ed25519hash.Sign(h.secretKey, renterhost.HashRevision(initRevision)),
	}
	var renterSigs renterhost.RPCFormContractSignatures
	s.sess.ReadResponse(&renterSigs, 4096)
	h.contracts[initRevision.ParentID] = &hostContract{
		rev:  initRevision,
		sigs: [2]types.TransactionSignature{renterSigs.RevisionSignature, hostRevisionSig},
	}
	hostSigs := &renterhost.RPCFormContractSignatures{RevisionSignature: hostRevisionSig}
	return s.sess.WriteResponse(hostSigs, nil)
}

func (h *Host) rpcLock(s *hostSession) error {
	var req renterhost.RPCLockRequest
	s.sess.ReadRequest(&req, 4096)
	s.contract = h.contracts[req.ContractID]
	var newChallenge [16]byte
	frand.Read(newChallenge[:])
	s.sess.SetChallenge(newChallenge)
	resp := &renterhost.RPCLockResponse{
		Acquired:     true,
		NewChallenge: newChallenge,
		Revision:     s.contract.rev,
		Signatures:   s.contract.sigs[:],
	}
	return s.sess.WriteResponse(resp, nil)
}

func (h *Host) rpcRenewContract(s *hostSession) error {
	var req renterhost.RPCRenewAndClearContractRequest
	s.sess.ReadRequest(&req, 4096)
	txn := req.Transactions[len(req.Transactions)-1]
	fc := txn.FileContracts[0]
	resp := &renterhost.RPCFormContractAdditions{}
	s.sess.WriteResponse(resp, nil)
	initRevision := types.FileContractRevision{
		ParentID: txn.FileContractID(0),
		UnlockConditions: types.UnlockConditions{
			PublicKeys:         []types.SiaPublicKey{req.RenterKey, h.PublicKey().SiaPublicKey()},
			SignaturesRequired: 2,
		},
		NewRevisionNumber:     1,
		NewFileSize:           fc.FileSize,
		NewFileMerkleRoot:     fc.FileMerkleRoot,
		NewWindowStart:        fc.WindowStart,
		NewWindowEnd:          fc.WindowEnd,
		NewValidProofOutputs:  fc.ValidProofOutputs,
		NewMissedProofOutputs: fc.MissedProofOutputs,
		NewUnlockHash:         fc.UnlockHash,
	}
	hostRevisionSig := types.TransactionSignature{
		ParentID:       crypto.Hash(initRevision.ParentID),
		CoveredFields:  types.CoveredFields{FileContractRevisions: []uint64{0}},
		PublicKeyIndex: 1,
		Signature:      ed25519hash.Sign(h.secretKey, renterhost.HashRevision(initRevision)),
	}
	var renterSigs renterhost.RPCRenewAndClearContractSignatures
	s.sess.ReadResponse(&renterSigs, 4096)
	h.contracts[initRevision.ParentID] = &hostContract{
		rev:  initRevision,
		sigs: [2]types.TransactionSignature{renterSigs.RevisionSignature, hostRevisionSig},
	}
	hostSigs := &renterhost.RPCRenewAndClearContractSignatures{
		RevisionSignature: hostRevisionSig,
	}
	return s.sess.WriteResponse(hostSigs, nil)
}
