package main

import (
	"context"
	"log"
	"os/user"
	"path/filepath"
	"runtime"
	"time"

	"github.com/BurntSushi/toml"
	"go.sia.tech/siad/build"
	"lukechampine.com/flagg"
	"lukechampine.com/us/hostdb"
	"lukechampine.com/us/renter"
)

var (
	// to be supplied at build time
	githash   = "?"
	builddate = "?"
)

var (
	rootUsage = `Usage:
    musec [flags] [action]

Actions:
    scan            scan a host
    form            form a contract
    renew           renew a contract
    contracts       list all contracts
    hosts           view and create host sets
    checkup         check the health of a contract
    info            display info about a contract
`
	versionUsage = rootUsage
	scanUsage    = `Usage:
    musec scan hostkey bytes duration

Scans the specified host and reports various metrics.

bytes is the number of bytes intended to be stored on the host; duration is
the number of blocks that the contract will be active.
`
	formUsage = `Usage:
    musec form hostkey funds duration
    musec form hostkey funds @endheight

Forms a contract with the specified host for the specified duration with the
specified amount of funds. To specify an exact end height for the contract,
use @endheight; otherwise, the end height will be the current height plus the
supplied duration. Due to various fees, the total number of coins deducted
from the wallet may be greater than funds. Run 'musec scan' on the host to see
a breakdown of these fees.
`
	renewUsage = `Usage:
    musec renew contract funds duration
    musec renew contract funds @endheight
    musec renew contract funds +extension

Renews the contract with the specified ID for the specified duration and with
the specified amount of funds. Like 'musec form', an exact end height can be
specified using the @ prefix; additionally, a + prefix will set the end height
equal to the old contract end height plus the supplied extension. Due to various
fees, the total number of coins deducted from the wallet may be greater than
funds. Run 'musec scan' on the host to see a breakdown of these fees.
`
	checkupUsage = `Usage:
    musec checkup contract

Verifies that a randomly-selected sector of the contract with the specified ID
is retrievable, and reports the resulting performance and price metrics.
Note that this operation is not free!
`
	contractsUsage = `Usage:
    musec contracts [host set]

Lists contracts, along with various metadata. If a host set is provided,
only the contracts formed with those hosts are listed.
`
	hostsUsage = `Usage:
    musec hosts [action]

Actions:
	create          create a host set
	delete          delete a host set
	add             add a host to a host set
	remove          remove a host from a host set

Lists host sets.
`
	hostsCreateUsage = `Usage:
musec hosts create [name] [host set]

Creates or replaces the host set with the given name. The host set is
specified as a comma-separated list of pubkeys.
`
	hostsDeleteUsage = `Usage:
musec hosts delete [name]

Deletes the host set with the given name.
`
	hostsAddUsage = `Usage:
musec hosts add [name] [host]

Adds a host to the host set with the given name.
`
	hostsRemoveUsage = `Usage:
musec hosts remove [name] [host]

Removes a host from the host set with the given name.
`
	infoUsage = `Usage:
    musec info contract

Displays metadata about the contract with the specified ID.
`
)

var usage = flagg.SimpleUsage(flagg.Root, rootUsage) // point-free style!

func check(ctx string, err error) {
	if err != nil {
		log.Fatalln(ctx, err)
	}
}

func scanHost(hkr renter.HostKeyResolver, pubkey hostdb.HostPublicKey) (hostdb.ScannedHost, error) {
	addr, err := hkr.ResolveHostKey(pubkey)
	if err != nil {
		return hostdb.ScannedHost{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return hostdb.Scan(ctx, addr, pubkey)
}

func loadAddrFromConfig() string {
	user, err := user.Current()
	if err != nil {
		return ""
	}
	var config struct {
		MuseAddr string `toml:"muse_addr"`
	}
	toml.DecodeFile(filepath.Join(user.HomeDir, ".config", "user", "config.toml"), &config)
	return config.MuseAddr
}

func main() {
	log.SetFlags(0)

	// read server addr from user config file, if it exists
	museAddr := loadAddrFromConfig()

	rootCmd := flagg.Root
	rootCmd.StringVar(&museAddr, "a", museAddr, "host:port that the muse API is running on")
	rootCmd.Usage = flagg.SimpleUsage(rootCmd, rootUsage)

	versionCmd := flagg.New("version", versionUsage)
	scanCmd := flagg.New("scan", scanUsage)
	formCmd := flagg.New("form", formUsage)
	renewCmd := flagg.New("renew", renewUsage)
	checkupCmd := flagg.New("checkup", checkupUsage)
	contractsCmd := flagg.New("contracts", contractsUsage)
	hostsCmd := flagg.New("hosts", hostsUsage)
	hostsCreateCmd := flagg.New("create", hostsCreateUsage)
	hostsDeleteCmd := flagg.New("delete", hostsDeleteUsage)
	hostsAddCmd := flagg.New("add", hostsAddUsage)
	hostsRemoveCmd := flagg.New("remove", hostsRemoveUsage)
	infoCmd := flagg.New("info", infoUsage)

	cmd := flagg.Parse(flagg.Tree{
		Cmd: rootCmd,
		Sub: []flagg.Tree{
			{Cmd: versionCmd},
			{Cmd: scanCmd},
			{Cmd: formCmd},
			{Cmd: renewCmd},
			{Cmd: checkupCmd},
			{Cmd: contractsCmd},
			{Cmd: hostsCmd, Sub: []flagg.Tree{
				{Cmd: hostsCreateCmd},
				{Cmd: hostsAddCmd},
				{Cmd: hostsRemoveCmd},
				{Cmd: hostsDeleteCmd},
			}},
			{Cmd: infoCmd},
		},
	})
	args := cmd.Args()

	switch cmd {
	case rootCmd:
		if len(args) > 0 {
			usage()
			return
		}
		fallthrough
	case versionCmd:
		log.Printf("musec v0.4.0\nCommit:     %s\nRelease:    %s\nGo version: %s %s/%s\nBuild Date: %s\n",
			githash, build.Release, runtime.Version(), runtime.GOOS, runtime.GOARCH, builddate)

	case scanCmd:
		hostkey, bytes, duration := parseScan(args, scanCmd)
		err := scan(museAddr, hostkey, bytes, duration)
		check("Scan failed:", err)

	case formCmd:
		host, funds, end := parseForm(args, formCmd)
		err := form(museAddr, host, funds, end)
		check("Contract formation failed:", err)

	case renewCmd:
		contract, funds, end := parseRenew(args, renewCmd)
		err := renew(museAddr, contract, funds, end)
		check("Renew failed:", err)

	case checkupCmd:
		if len(args) != 1 {
			cmd.Usage()
			return
		}
		err := checkup(museAddr, args[0])
		check("Checkup failed:", err)

	case contractsCmd:
		hostset := parseContracts(args, contractsCmd)
		err := listContracts(museAddr, hostset)
		check("Could not list contracts:", err)

	case hostsCmd:
		if len(args) != 0 {
			cmd.Usage()
			return
		}
		err := listHosts(museAddr)
		check("Could not list host sets:", err)

	case hostsCreateCmd:
		name, set := parseHostsCreate(args, hostsCreateCmd)
		err := createHostSet(museAddr, name, set)
		check("Could not create or update host set:", err)

	case hostsDeleteCmd:
		if len(args) != 1 {
			cmd.Usage()
			return
		}
		err := deleteHostSet(museAddr, args[0])
		check("Could not delete host set:", err)

	case hostsAddCmd:
		if len(args) != 2 {
			cmd.Usage()
			return
		}
		err := addHost(museAddr, args[0], args[1])
		check("Could not add host:", err)

	case hostsRemoveCmd:
		if len(args) != 2 {
			cmd.Usage()
			return
		}
		err := removeHost(museAddr, args[0], args[1])
		check("Could not remove host:", err)

	case infoCmd:
		if len(args) != 1 {
			infoCmd.Usage()
			return
		}
		err := info(museAddr, args[0])
		check("Could not get contract info:", err)
	}
}
