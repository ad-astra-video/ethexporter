package main

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	allWatching []*Watching
	port        string
	prefix      string
	loadSeconds float64
	totalLoaded int64
	eth         *ethclient.Client
)

type Watching struct {
	Name    string
	Address string
	Balance string
}

func ConnectionToGeth(url string) error {
	var err error
	eth, err = ethclient.Dial(url)
	return err
}

func GetEthBalance(address string) *big.Float {
	balance, err := eth.BalanceAt(context.TODO(), common.HexToAddress(address), nil)
	if err != nil {
		fmt.Printf("Error fetching ETH Balance for address: %v\n", address)
	}
	return ToEther(balance)
}

func CurrentBlock() uint64 {
	block, err := eth.BlockByNumber(context.TODO(), nil)
	if err != nil {
		fmt.Printf("Error fetching current block height: %v\n", err)
		return 0
	}
	return block.NumberU64()
}

func ToEther(o *big.Int) *big.Float {
	val := new(big.Float).SetInt(o)
	return new(big.Float).Mul(val, big.NewFloat(1e-18))
}

func MetricsHttp(w http.ResponseWriter, r *http.Request) {
	var allOut []string
	total := big.NewFloat(0)
	for _, v := range allWatching {
		if v.Balance == "" {
			v.Balance = "0"
		}
		bal := new(big.Float)
		bal.SetString(v.Balance)
		total.Add(total, bal)
		allOut = append(allOut, fmt.Sprintf("%veth_balance{name=\"%v\",address=\"%v\"} %v", prefix, v.Name, v.Address, v.Balance))
	}
	allOut = append(allOut, fmt.Sprintf("%veth_balance_total %0.18f", prefix, total))
	allOut = append(allOut, fmt.Sprintf("%veth_load_seconds %0.2f", prefix, loadSeconds))
	allOut = append(allOut, fmt.Sprintf("%veth_loaded_addresses %v", prefix, totalLoaded))
	allOut = append(allOut, fmt.Sprintf("%veth_total_addresses %v", prefix, len(allWatching)))
	fmt.Fprintln(w, strings.Join(allOut, "\n"))
}

// Parse addresses from ADDRESSES env variable (format: name:0xabc,name2:0xdef)
func LoadAddressesFromEnv() error {
	addressEnv := os.Getenv("ADDRESSES")
	if addressEnv == "" {
		return fmt.Errorf("ADDRESSES environment variable not set")
	}

	items := strings.Split(addressEnv, ",")
	for _, item := range items {
		parts := strings.Split(item, ":")
		if len(parts) != 2 {
			fmt.Printf("Skipping invalid address entry: %v\n", item)
			continue
		}
		name, addr := parts[0], parts[1]
		if !common.IsHexAddress(addr) {
			fmt.Printf("Skipping invalid Ethereum address: %v\n", addr)
			continue
		}
		allWatching = append(allWatching, &Watching{Name: name, Address: addr})
	}

	if len(allWatching) == 0 {
		return fmt.Errorf("no valid addresses found in ADDRESSES")
	}
	return nil
}

func main() {
	gethUrl := os.Getenv("GETH")
	port = os.Getenv("PORT")
	prefix = os.Getenv("PREFIX")

	if err := LoadAddressesFromEnv(); err != nil {
		panic(err)
	}

	if err := ConnectionToGeth(gethUrl); err != nil {
		panic(err)
	}

	// check address balances
	go func() {
		for {
			totalLoaded = 0
			t1 := time.Now()
			fmt.Printf("Checking %v wallets...\n", len(allWatching))
			for _, v := range allWatching {
				v.Balance = GetEthBalance(v.Address).String()
				totalLoaded++
			}
			loadSeconds = time.Since(t1).Seconds()
			fmt.Printf("Finished checking %v wallets in %.0f seconds, sleeping for 15 seconds.\n", len(allWatching), loadSeconds)
			time.Sleep(15 * time.Second)
		}
	}()

	block := CurrentBlock()
	fmt.Printf("ETHexporter started on port %v using Geth: %v at block #%v\n", port, gethUrl, block)
	http.HandleFunc("/metrics", MetricsHttp)
	panic(http.ListenAndServe("0.0.0.0:"+port, nil))
}
