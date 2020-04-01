package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"gitlab.com/NebulousLabs/Sia/build"
	"gitlab.com/NebulousLabs/Sia/modules"
	"gitlab.com/NebulousLabs/Sia/modules/consensus"
	"gitlab.com/NebulousLabs/Sia/modules/gateway"
	"gitlab.com/NebulousLabs/Sia/modules/transactionpool"
	"golang.org/x/crypto/ssh/terminal"
	"lukechampine.com/muse"
	"lukechampine.com/shard"
	"lukechampine.com/us/wallet"
	"lukechampine.com/walrus"
)

var (
	// to be supplied at build time
	githash   = "?"
	builddate = "?"
)

func getSeed() wallet.Seed {
	phrase := os.Getenv("WALRUS_SEED")
	if phrase != "" {
		fmt.Println("Using WALRUS_SEED environment variable")
	} else {
		fmt.Print("Seed: ")
		pw, err := terminal.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			log.Fatal("Could not read seed phrase:", err)
		}
		fmt.Println()
		phrase = string(pw)
	}
	seed, err := wallet.SeedFromPhrase(phrase)
	if err != nil {
		log.Fatal(err)
	}
	return seed
}

func main() {
	log.SetFlags(0)
	apiAddr := flag.String("a", ":9580", "host:port that the API server listens on")
	walrusAddr := flag.String("w", "", "host:port of the walrus server (optional)")
	shardAddr := flag.String("s", "", "host:port of the shard server (optional)")
	dir := flag.String("d", ".", "directory where server state is stored")
	flag.Parse()

	if len(flag.Args()) == 1 && flag.Arg(0) == "version" {
		log.Printf("muse v0.6.0\nCommit:     %s\nRelease:    %s\nGo version: %s %s/%s\nBuild Date: %s\n",
			githash, build.Release, runtime.Version(), runtime.GOOS, runtime.GOARCH, builddate)
		return
	} else if len(flag.Args()) != 0 {
		flag.Usage()
		return
	}

	// if server addrs provided, use them; otherwise, run full node
	switch haveWalrus, haveShard := *walrusAddr != "", *shardAddr != ""; {
	case haveWalrus, haveShard:
		log.Println("Using provided walrus and shard servers")
		log.Println("Running in lite client mode...")

	case haveWalrus:
		log.Println("Using provided walrus server; no shard server provided")
		log.Println("Running in full node mode...")
		var err error
		if *shardAddr, err = createShardServer(*dir); err != nil {
			log.Fatal("Couldn't initialize full node:", err)
		}

	case haveShard:
		log.Println("Using provided shard server; no walrus server provided")
		log.Println("Running in full node mode...")
		var err error
		if *walrusAddr, err = createWalletServer(*dir); err != nil {
			log.Fatal("Couldn't initialize full node:", err)
		}

	default:
		log.Println("No walrus or shard server provided")
		log.Println("Running in full node mode...")
		var err error
		if *walrusAddr, err = createWalletServer(*dir); err != nil {
			log.Fatal("Couldn't initialize full node:", err)
		}
		if *shardAddr, err = createShardServer(*dir); err != nil {
			log.Fatal("Couldn't initialize full node:", err)
		}
	}

	wc := walrus.NewClient(*walrusAddr)
	srv, err := muse.NewServer(*dir, wc.ProtoWallet(getSeed()), wc.ProtoTransactionPool(), *shardAddr)
	if err != nil {
		log.Fatal("Could not initialize server:", err)
	}

	log.Printf("Listening on %v...", *apiAddr)
	log.Fatal(http.ListenAndServe(*apiAddr, srv))
}

// global vars to make it easier to compose createShardServer and createWallet
// (yeah yeah, sue me)
var (
	g  modules.Gateway
	cs modules.ConsensusSet
)

func createShardServer(dir string) (addr string, err error) {
	if g == nil {
		g, err = gateway.New(":9381", true, filepath.Join(dir, "gateway"))
		if err != nil {
			return "", err
		}
	}
	if cs == nil {
		var errChan <-chan error
		cs, errChan = consensus.New(g, true, filepath.Join(dir, "consensus"))
		err = handleAsyncErr(errChan)
		if err != nil {
			return "", err
		}
	}
	// muse expects a shard URL, not an interface, so start up a server and
	// return the address it's listening on. This is kind of gross.
	r, err := shard.NewRelay(cs, shard.NewJSONPersist(dir))
	l, err := net.Listen("tcp", "localhost:0")
	go http.Serve(l, shard.NewServer(r))
	return "http://" + l.Addr().String(), nil
}

func createWalletServer(dir string) (addr string, err error) {
	if g == nil {
		g, err = gateway.New(":9381", true, filepath.Join(dir, "gateway"))
		if err != nil {
			return "", err
		}
	}
	if cs == nil {
		var errChan <-chan error
		cs, errChan = consensus.New(g, true, filepath.Join(dir, "consensus"))
		err = handleAsyncErr(errChan)
		if err != nil {
			return "", err
		}
	}
	tp, err := transactionpool.New(cs, g, filepath.Join(dir, "tpool"))
	if err != nil {
		return "", err
	}
	store, err := wallet.NewBoltDBStore(filepath.Join(dir, "wallet.db"), nil)
	if err != nil {
		return "", err
	}
	w := wallet.New(store)
	err = cs.ConsensusSetSubscribe(w.ConsensusSetSubscriber(store), store.ConsensusChangeID(), nil)

	// same grossness as above
	l, err := net.Listen("tcp", "localhost:0")
	go http.Serve(l, walrus.NewServer(w, tp))
	return "http://" + l.Addr().String(), nil
}

func handleAsyncErr(errCh <-chan error) error {
	select {
	case err := <-errCh:
		return err
	default:
	}
	go func() {
		err := <-errCh
		if err != nil {
			log.Println("WARNING: consensus initialization returned an error:", err)
		}
	}()
	return nil
}
