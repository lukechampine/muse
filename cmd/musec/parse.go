package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"math/big"
	"os"
	"strconv"
	"strings"

	"go.sia.tech/siad/types"
)

// form [hostkey] [funds] [endheight/duration]
func parseForm(args []string, cmd *flag.FlagSet) (string, types.Currency, string) {
	if len(args) != 3 {
		cmd.Usage()
		os.Exit(2)
	}
	return args[0], parseCurrency(args[1]), args[2]
}

// renew [contract] [funds] [endheight/duration]
func parseRenew(args []string, cmd *flag.FlagSet) (string, types.Currency, string) {
	if len(args) != 3 {
		cmd.Usage()
		os.Exit(2)
	}
	return args[0], parseCurrency(args[1]), args[2]
}

// contracts
// contracts [host set]
func parseContracts(args []string, cmd *flag.FlagSet) string {
	if len(args) > 1 {
		cmd.Usage()
		os.Exit(2)
	}
	args = append(args, "")
	return args[0]
}

// scan [hostkey] [bytes] [duration]
func parseScan(args []string, cmd *flag.FlagSet) (string, uint64, types.BlockHeight) {
	if len(args) != 3 {
		cmd.Usage()
		os.Exit(2)
	}
	return args[0], parseFilesize(args[1]), parseBlockHeight(args[2])
}

// create [name] [host set]
func parseHostsCreate(args []string, cmd *flag.FlagSet) (string, []string) {
	if len(args) != 2 {
		cmd.Usage()
		os.Exit(2)
	}
	return args[0], strings.Split(args[1], ",")
}

func parseCurrency(s string) types.Currency {
	var hastings string
	if strings.HasSuffix(s, "H") {
		hastings = strings.TrimSuffix(s, "H")
	} else {
		units := []string{"pS", "nS", "uS", "mS", "SC", "KS", "MS", "GS", "TS"}
		for i, unit := range units {
			if strings.HasSuffix(s, unit) {
				// scan into big.Rat
				r, ok := new(big.Rat).SetString(strings.TrimSuffix(s, unit))
				if !ok {
					log.Fatal("Malformed currency value")
				}
				// convert units
				exp := 24 + 3*(int64(i)-4)
				mag := new(big.Int).Exp(big.NewInt(10), big.NewInt(exp), nil)
				r.Mul(r, new(big.Rat).SetInt(mag))
				// r must be an integer at this point
				if !r.IsInt() {
					log.Fatal("Non-integer number of hastings")
				}
				hastings = r.RatString()
				break
			}
		}
	}
	if hastings == "" {
		log.Fatal("Currency value is missing units")
	}
	var c types.Currency
	_, err := fmt.Sscan(hastings, &c)
	check("Could not scan currency value:", err)
	return c
}

func parseBlockHeight(s string) types.BlockHeight {
	height, err := strconv.Atoi(s)
	check("Malformed blockheight:", err)
	return types.BlockHeight(height)
}

func parseFilesize(s string) (bytes uint64) {
	units := []struct {
		suffix     string
		multiplier uint64
	}{
		{"kb", 1e3},
		{"mb", 1e6},
		{"gb", 1e9},
		{"tb", 1e12},
		{"kib", 1 << 10},
		{"mib", 1 << 20},
		{"gib", 1 << 30},
		{"tib", 1 << 40},
		{"b", 1},
	}

	s = strings.ToLower(s)
	for _, unit := range units {
		if strings.HasSuffix(s, unit.suffix) {
			_, err := fmt.Sscan(s, &bytes)
			check("Malformed filesize:", err)
			bytes *= unit.multiplier
			return
		}
	}

	// no units
	_, err := fmt.Sscan(s, &bytes)
	check("Malformed filesize:", err)
	return
}

func parseEnd(startHeight types.BlockHeight, end string) (types.BlockHeight, error) {
	if end == "" {
		return 0, errors.New("invalid end height or duration")
	}
	switch end[0] {
	case '@':
		intHeight, err := strconv.Atoi(end[1:])
		if err != nil {
			return 0, err
		}
		return types.BlockHeight(intHeight), nil
	default:
		intDuration, err := strconv.Atoi(end)
		if err != nil {
			return 0, err
		}
		return startHeight + types.BlockHeight(intDuration), nil
	}
}

func currencyUnits(c types.Currency) string {
	atto := types.NewCurrency64(1000000)
	if c.Cmp(atto) < 0 {
		return c.String() + " H"
	}
	mag := atto
	unit := ""
	for _, unit = range []string{"aS", "fS", "pS", "nS", "uS", "mS", "SC", "KS", "MS", "GS", "TS"} {
		if c.Cmp(mag.Mul64(1e3)) < 0 {
			break
		} else if unit != "TS" {
			mag = mag.Mul64(1e3)
		}
	}
	num := new(big.Rat).SetInt(c.Big())
	denom := new(big.Rat).SetInt(mag.Big())
	res, _ := new(big.Rat).Mul(num, denom.Inv(denom)).Float64()
	return fmt.Sprintf("%.4g %s", res, unit)
}

func filesizeUnits(size int64) string {
	if size == 0 {
		return "0 B"
	}
	sizes := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"}
	i := int(math.Log10(float64(size)) / 3)
	// printf trick: * means "print to 'i' digits"
	// so we get 1 decimal place for KB, 2 for MB, 3 for GB, etc.
	return fmt.Sprintf("%.*f %s", i, float64(size)/math.Pow10(3*i), sizes[i])
}
