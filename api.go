// Package muse provides a Sia contract server and client.
package muse // import "lukechampine.com/muse"

import (
	"crypto/ed25519"
	"encoding/json"

	"gitlab.com/NebulousLabs/Sia/modules"
	"gitlab.com/NebulousLabs/Sia/types"
	"lukechampine.com/us/hostdb"
	"lukechampine.com/us/renter"
)

// A Contract represents a Sia file contract, along with additional metadata.
type Contract struct {
	renter.Contract
	HostAddress modules.NetAddress
	EndHeight   types.BlockHeight
}

type responseContracts []Contract

// MarshalJSON implements json.Marshaler.
func (r responseContracts) MarshalJSON() ([]byte, error) {
	enc := make([]struct {
		HostKey     hostdb.HostPublicKey `json:"hostKey"`
		ID          types.FileContractID `json:"id"`
		RenterKey   ed25519.PrivateKey   `json:"renterKey"`
		HostAddress modules.NetAddress   `json:"hostAddress"`
		EndHeight   types.BlockHeight    `json:"endHeight"`
	}, len(r))
	for i := range enc {
		enc[i].HostKey = r[i].HostKey
		enc[i].ID = r[i].ID
		enc[i].RenterKey = r[i].RenterKey
		enc[i].HostAddress = r[i].HostAddress
		enc[i].EndHeight = r[i].EndHeight
	}
	return json.Marshal(enc)
}

// RequestForm is the request type for the /form endpoint.
type RequestForm struct {
	HostKey     hostdb.HostPublicKey
	Funds       types.Currency
	StartHeight types.BlockHeight
	EndHeight   types.BlockHeight
	Settings    hostdb.HostSettings
}

// RequestRenew is the request type for the /renew endpoint.
type RequestRenew struct {
	ID          types.FileContractID
	Funds       types.Currency
	StartHeight types.BlockHeight
	EndHeight   types.BlockHeight
	Settings    hostdb.HostSettings
}

// RequestScan is the request type for the /scan endpoint.
type RequestScan struct {
	HostKey hostdb.HostPublicKey
}
