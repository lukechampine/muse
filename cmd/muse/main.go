package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"

	"gitlab.com/NebulousLabs/Sia/build"
	"golang.org/x/crypto/ssh/terminal"
	"lukechampine.com/muse"
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
	walrusAddr := flag.String("w", "", "host:port of the walrus server")
	shardAddr := flag.String("s", "", "host:port of the shard server")
	dir := flag.String("d", ".", "directory where server state is stored")
	flag.Parse()

	if len(flag.Args()) == 1 && flag.Arg(0) == "version" {
		log.Printf("muse v0.1.0\nCommit:     %s\nRelease:    %s\nGo version: %s %s/%s\nBuild Date: %s\n",
			githash, build.Release, runtime.Version(), runtime.GOOS, runtime.GOARCH, builddate)
		return
	} else if len(flag.Args()) != 0 {
		flag.Usage()
		return
	}
	if *walrusAddr == "" {
		log.Fatal("No walrus address provided")
	} else if *shardAddr == "" {
		log.Fatal("No shard address provided")
	}

	w := walrus.NewClient(*walrusAddr)
	srv, err := muse.NewServer(*dir, w.ProtoWallet(getSeed()), w.ProtoTransactionPool(), *shardAddr)
	if err != nil {
		log.Fatal("Could not initialize server:", err)
	}

	log.Printf("Listening on %v...", *apiAddr)
	log.Fatal(http.ListenAndServe(*apiAddr, srv))
}
