package muse

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"

	"gitlab.com/NebulousLabs/Sia/types"
	"lukechampine.com/shard"
	"lukechampine.com/us/hostdb"
)

// A Client communicates with a muse server.
type Client struct {
	addr string
}

func (c *Client) req(ctx context.Context, method string, route string, data, resp interface{}) error {
	var body io.Reader
	if data != nil {
		js, _ := json.Marshal(data)
		body = bytes.NewReader(js)
	}
	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("%v%v", c.addr, route), body)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	defer io.Copy(ioutil.Discard, r.Body)
	if r.StatusCode != 200 {
		err, _ := ioutil.ReadAll(r.Body)
		return errors.New(strings.TrimSpace(string(err)))
	}
	if resp == nil {
		return nil
	}
	return json.NewDecoder(r.Body).Decode(resp)
}

func (c *Client) get(ctx context.Context, route string, r interface{}) error {
	return c.req(ctx, "GET", route, nil, r)
}
func (c *Client) post(ctx context.Context, route string, d, r interface{}) error {
	return c.req(ctx, "POST", route, d, r)
}
func (c *Client) put(ctx context.Context, route string, d, r interface{}) error {
	return c.req(ctx, "PUT", route, d, r)
}

// AllContracts returns all contracts formed by the server.
func (c *Client) AllContracts(ctx context.Context) (cs []Contract, err error) {
	err = c.get(ctx,"/contracts", &cs)
	return
}

// Contracts returns the contracts in the specified set.
func (c *Client) Contracts(ctx context.Context, set string) (cs []Contract, err error) {
	if set == "" {
		return nil, errors.New("no host set provided; to retrieve all contracts, use AllContracts")
	}
	err = c.get(ctx,"/contracts?hostset="+set, &cs)
	return
}

// Scan queries the specified host for its current settings.
//
// Note that the host may also be scanned via the hostdb.Scan function.
func (c *Client) Scan(ctx context.Context, host hostdb.HostPublicKey) (settings hostdb.HostSettings, err error) {
	err = c.post(ctx,"/scan", RequestScan{
		HostKey: host,
	}, &settings)
	return
}

// Form forms a contract with a host. The settings should be obtained from a
// recent call to Scan. If the settings have changed in the interim, the host
// may reject the contract.
func (c *Client) Form(ctx context.Context, host hostdb.HostPublicKey, funds types.Currency, start, end types.BlockHeight, settings hostdb.HostSettings) (contract Contract, err error) {
	err = c.post(ctx,"/form", RequestForm{
		HostKey:     host,
		Funds:       funds,
		StartHeight: start,
		EndHeight:   end,
		Settings:    settings,
	}, &contract)
	return
}

// Renew renews the contract with the specified ID, which must refer to a
// contract previously formed by the server. The settings should be obtained
// from a recent call to Scan. If the settings have changed in the interim, the
// host may reject the contract.
func (c *Client) Renew(ctx context.Context, id types.FileContractID, funds types.Currency, start, end types.BlockHeight, settings hostdb.HostSettings) (contract Contract, err error) {
	err = c.post(ctx,"/renew", RequestRenew{
		ID:          id,
		Funds:       funds,
		StartHeight: start,
		EndHeight:   end,
		Settings:    settings,
	}, &contract)
	return
}

// HostSets returns the current list of host sets.
func (c *Client) HostSets(ctx context.Context) (hs []string, err error) {
	err = c.get(ctx,"/hostsets/", &hs)
	return
}

// HostSet returns the contents of the named host set.
func (c *Client) HostSet(ctx context.Context, name string) (hosts []hostdb.HostPublicKey, err error) {
	err = c.get(ctx,"/hostsets/"+name, &hosts)
	return
}

// SetHostSet sets the contents of a host set, creating it if it does not exist.
// If an empty slice is passed, the host set is deleted.
func (c *Client) SetHostSet(ctx context.Context, name string, hosts []hostdb.HostPublicKey) (err error) {
	err = c.put(ctx,"/hostsets/"+name, hosts, nil)
	return
}

// SHARD returns a client for the muse server's shard endpoints.
func (c *Client) SHARD() *shard.Client {
	u, err := url.Parse(c.addr)
	if err != nil {
		panic(err)
	}
	u.Path = path.Join(u.Path, "shard")
	return shard.NewClient(u.String())
}

// NewClient returns a client that communicates with a muse server listening
// on the specified address.
func NewClient(addr string) *Client {
	return &Client{addr}
}

func modifyURL(str string, fn func(*url.URL)) string {
	u, err := url.Parse(str)
	if err != nil {
		panic(err)
	}
	fn(u)
	return u.String()
}
