package main

import (
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/pkg/errors"
	"gitlab.com/NebulousLabs/Sia/types"
	"lukechampine.com/frand"
	"lukechampine.com/muse"
	"lukechampine.com/us/hostdb"
	"lukechampine.com/us/renter/proto"
	"lukechampine.com/us/renterhost"
)

func form(museAddr, hostPrefix string, funds types.Currency, endStr string) error {
	mc := muse.NewClient(museAddr)
	sc := mc.SHARD()
	start, err := sc.ChainHeight()
	if err != nil {
		return err
	}
	end, err := parseEnd(start, endStr)
	if err != nil {
		return err
	}
	hostKey, err := sc.LookupHost(hostPrefix)
	if err != nil {
		return err
	}
	settings, err := mc.Scan(hostKey)
	if err != nil {
		return err
	}
	c, err := mc.Form(hostKey, funds, start, end, settings)
	if err != nil {
		return err
	}
	fmt.Println("Formed contract", c.ID)
	return nil
}

func renew(museAddr, id string, funds types.Currency, endStr string) error {
	mc := muse.NewClient(museAddr)
	sc := mc.SHARD()

	var fcid types.FileContractID
	if err := fcid.LoadString(id); err != nil {
		return err
	}

	start, err := sc.ChainHeight()
	if err != nil {
		return err
	}
	end, err := parseEnd(start, endStr)
	if err != nil {
		return err
	}

	contracts, err := mc.AllContracts()
	if err != nil {
		return err
	}
	var hostKey hostdb.HostPublicKey
	for _, c := range contracts {
		if c.ID == fcid {
			hostKey = c.HostKey
			break
		}
	}
	if hostKey == "" {
		return errors.New("no record of that contract")
	}

	settings, err := mc.Scan(hostKey)
	if err != nil {
		return err
	}
	rc, err := mc.Renew(fcid, funds, start, end, settings)
	if err != nil {
		return err
	}
	fmt.Println("Renewed contract:", rc.ID)
	return nil
}

func listContracts(museAddr, hostset string) error {
	c := muse.NewClient(museAddr)
	var contracts []muse.Contract
	var err error
	if hostset != "" {
		contracts, err = c.Contracts(hostset)
	} else {
		contracts, err = c.AllContracts()
	}
	if err != nil {
		return err
	}
	if len(contracts) == 0 {
		fmt.Println("No contracts.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Host:\tContract ID:\tEnd Height:\tIP Address:")
	for _, contract := range contracts {
		fmt.Fprintf(w, "%s\t%s\t%v\t%s\n", contract.HostKey.ShortKey(), contract.ID, contract.EndHeight, contract.HostAddress)
	}
	w.Flush()
	return nil
}

func listHosts(museAddr string) error {
	c := muse.NewClient(museAddr)
	sets, err := c.HostSets()
	if err != nil {
		return err
	}
	if len(sets) == 0 {
		fmt.Println("No host sets.")
		return nil
	}
	for _, setName := range sets {
		set, err := c.HostSet(setName)
		if err != nil {
			return err
		}
		fmt.Printf("%s (%v hosts):", setName, len(set))
		for i := range set {
			if i%5 == 0 {
				fmt.Printf("\n  ")
			}
			fmt.Print(set[i].ShortKey(), "  ")
		}
		fmt.Println()
	}
	return nil
}

func createHostSet(museAddr string, setName string, hostPrefixes []string) error {
	c := muse.NewClient(museAddr)
	sc := c.SHARD()
	hosts := make([]hostdb.HostPublicKey, len(hostPrefixes))
	for i := range hosts {
		var err error
		hosts[i], err = sc.LookupHost(hostPrefixes[i])
		if err != nil {
			return err
		}
	}
	err := c.SetHostSet(setName, hosts)
	if err != nil {
		return err
	}
	fmt.Printf("Created host set %q\n", setName)
	return nil
}

func deleteHostSet(museAddr string, setName string) error {
	c := muse.NewClient(museAddr)
	err := c.SetHostSet(setName, nil)
	if err != nil {
		return err
	}
	fmt.Printf("Deleted host set %q\n", setName)
	return nil
}

func addHost(museAddr string, setName string, host string) error {
	c := muse.NewClient(museAddr)
	hostKey, err := c.SHARD().LookupHost(host)
	if err != nil {
		return err
	}
	hosts, err := c.HostSet(setName)
	if err != nil {
		return err
	}
	for _, h := range hosts {
		if h == hostKey {
			fmt.Printf("Host %v is already in host set %q\n", hostKey.ShortKey(), setName)
			return nil
		}
	}
	hosts = append(hosts, hostKey)
	err = c.SetHostSet(setName, hosts)
	if err != nil {
		return err
	}
	fmt.Printf("Added host %v to host set %q\n", hostKey.ShortKey(), setName)
	return nil
}

func removeHost(museAddr string, setName string, host string) error {
	c := muse.NewClient(museAddr)
	hostKey, err := c.SHARD().LookupHost(host)
	if err != nil {
		return err
	}
	hosts, err := c.HostSet(setName)
	if err != nil {
		return err
	}
	for i, h := range hosts {
		if h == hostKey {
			hosts = append(hosts[:i], hosts[i+1:]...)
			err = c.SetHostSet(setName, hosts)
			if err != nil {
				return err
			}
			fmt.Printf("Removed host %v from host set %q\n", hostKey.ShortKey(), setName)
			return nil
		}
	}
	fmt.Printf("Host %v is not in host set %q\n", hostKey.ShortKey(), setName)
	return nil
}

func scan(museAddr, hostKeyPrefix string, bytes uint64, duration types.BlockHeight) error {
	c := muse.NewClient(museAddr)
	sc := c.SHARD()

	currentHeight, err := sc.ChainHeight()
	if err != nil {
		return errors.Wrap(err, "could not get current height")
	}
	hostKey, err := sc.LookupHost(hostKeyPrefix)
	if err != nil {
		return errors.Wrap(err, "could not lookup host")
	}
	host, err := c.Scan(hostKey)
	if err != nil {
		return errors.Wrap(err, "could not scan host")
	}

	if !host.AcceptingContracts {
		fmt.Printf("Warning: host is not accepting contracts\n")
	} else if host.RemainingStorage < bytes {
		fmt.Printf("Warning: host reports only %v of remaining storage\n", filesizeUnits(int64(host.RemainingStorage)))
	}

	storageCost := host.StoragePrice.Mul64(bytes).Mul64(uint64(duration))
	uploadCost := host.UploadBandwidthPrice.Mul64(bytes)
	downloadCost := host.DownloadBandwidthPrice.Mul64(bytes)
	hostFee := host.ContractPrice
	renterFunds := storageCost.Add(uploadCost).Add(downloadCost).Add(hostFee)
	hostCollateral := host.Collateral.Mul64(bytes).Mul64(uint64(duration))
	if hostCollateral.Cmp(host.MaxCollateral) > 0 {
		hostCollateral = host.MaxCollateral
	}
	totalInputs := renterFunds.Add(hostCollateral)
	siafundFee := types.Tax(currentHeight, totalInputs)
	costToRenter := renterFunds.Add(siafundFee)

	bar := func(c, max types.Currency) string {
		pct, _ := c.Mul64(500).Div(max).Uint64()
		if pct == 0 {
			return "▏"
		}
		str := strings.Repeat("█", int(pct/8))
		if pct%8 != 0 {
			str += string(rune('█' + (8 - pct%8)))
		}
		return str
	}

	fmt.Printf(`
Public Key:       %v
IP Address:       %v

Cost Summary:
Storage Cost:     %8v %v
Upload Cost:      %8v %v
Download Cost:    %8v %v
Host Fee:         %8v %v
Siafund Fee:      %8v %v

Contract Inputs:
Renter Funds:     %8v %v
Host Collateral:  %8v %v
Collateral Ratio: %8v

Contract Value:   %8v (pass this value to the 'form' and 'renew' commands)
Transaction Cost: %8v (this is the amount that will be deducted from your wallet)
`,
		hostKey,
		host.NetAddress,

		currencyUnits(storageCost), bar(storageCost, costToRenter),
		currencyUnits(uploadCost), bar(uploadCost, costToRenter),
		currencyUnits(downloadCost), bar(downloadCost, costToRenter),
		currencyUnits(hostFee), bar(hostFee, costToRenter),
		currencyUnits(siafundFee), bar(siafundFee, costToRenter),

		currencyUnits(renterFunds), bar(renterFunds, totalInputs),
		currencyUnits(hostCollateral), bar(hostCollateral, totalInputs),
		new(big.Rat).SetFrac(hostCollateral.Big(), renterFunds.Big()).FloatString(2)+"x",

		currencyUnits(renterFunds),
		currencyUnits(costToRenter))

	return nil
}

func info(museAddr string, id string) error {
	c := muse.NewClient(museAddr)
	sc := c.SHARD()
	contracts, err := c.AllContracts()
	if err != nil {
		return err
	}
	var contract muse.Contract
	for _, contract = range contracts {
		if strings.HasPrefix(contract.ID.String(), id) {
			break
		}
	}
	if len(contract.RenterKey) == 0 {
		return errors.New("contract not found")
	}
	currentHeight, err := sc.ChainHeight()
	if err != nil {
		return err
	}

	funds, sectors, revErr := func() (string, string, error) {
		sess, err := proto.NewSession(contract.HostAddress, contract.HostKey, contract.ID, contract.RenterKey, currentHeight)
		if err != nil {
			return "?", "?", err
		}
		rev := sess.Revision()
		funds := fmt.Sprintf("%v remaining", currencyUnits(rev.RenterFunds()))
		sectors := fmt.Sprintf("%v (%v)", rev.Revision.NewFileSize/renterhost.SectorSize,
			filesizeUnits(int64(rev.Revision.NewFileSize)))
		return funds, sectors, sess.Close()
	}()

	var remaining string
	if currentHeight <= contract.EndHeight {
		remaining = fmt.Sprintf("(%v blocks remaining)", contract.EndHeight-currentHeight)
	} else {
		remaining = fmt.Sprintf("(expired %v blocks ago)", currentHeight-contract.EndHeight)
	}

	fmt.Printf(`Host Key:     %v
Host Address: %v
Contract ID:  %v
End Height:   %v %v
Renter Funds: %v
Sectors:      %v
`, contract.HostKey.Key(), contract.HostAddress, contract.ID, contract.EndHeight, remaining, funds, sectors)

	if revErr != nil {
		fmt.Println("\nSome values could not be determined because the host returned an error:\n ", revErr)
	}

	return nil
}

func checkup(museAddr string, id string) error {
	c := muse.NewClient(museAddr)
	sc := c.SHARD()
	contracts, err := c.AllContracts()
	if err != nil {
		return err
	}
	var contract muse.Contract
	for _, contract = range contracts {
		if strings.HasPrefix(contract.ID.String(), id) {
			break
		}
	}
	if len(contract.RenterKey) == 0 {
		return errors.New("contract not found")
	}

	hostIP, err := sc.ResolveHostKey(contract.HostKey)
	if err != nil {
		return errors.Wrap(err, "could not resolve host key")
	}

	start := time.Now()
	sess, err := proto.NewSession(hostIP, contract.HostKey, contract.ID, contract.RenterKey, 0)
	if err != nil {
		return errors.Wrap(err, "could not initiate download protocol")
	}
	defer sess.Close()
	latency := time.Since(start)

	// fetch a random sector root
	numSectors := sess.Revision().NumSectors()
	if numSectors == 0 {
		return errors.New("no sectors stored on host")
	}
	roots, err := sess.SectorRoots(frand.Intn(numSectors), 1)
	if err != nil {
		return errors.Wrap(err, "could not get a sector to test")
	}
	root := roots[0]

	start = time.Now()
	err = sess.Read(ioutil.Discard, []renterhost.RPCReadRequestSection{{
		MerkleRoot: root,
		Offset:     0,
		Length:     renterhost.SectorSize,
	}})
	bandTime := time.Since(start)
	if err != nil {
		return errors.Wrap(err, "could not download sector")
	}
	bandwidth := (renterhost.SectorSize * 8 / 1e6) / bandTime.Seconds()

	fmt.Printf("OK   Host %v: Latency %0.3fms, Bandwidth %0.3f Mbps\n",
		contract.HostKey.ShortKey(), latency.Seconds()*1000, bandwidth)

	return nil
}
