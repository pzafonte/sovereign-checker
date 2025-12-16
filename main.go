package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"sovereign-checker/api"
	"sovereign-checker/btc"
	"sovereign-checker/ln"
	"sovereign-checker/netx"
	"sovereign-checker/planner"
	"sovereign-checker/score"
)

func main() {
	mode := flag.String("mode", "cli", "cli or server")

	// Common
	address := flag.String("address", "", "bitcoin address to check (cli mode)")
	networkStr := flag.String("network", "testnet", "mainnet or testnet")
	feeFallback := flag.Uint64("feerate", 2, "fallback feerate in sats/vB (used if no node estimate)")
	feeLow := flag.Uint64("feelow", 2, "low-fee threshold (sats/vB) for consolidation planning")

	// Tor
	torSocks := flag.String("tor", "", "tor SOCKS5 addr (e.g. 127.0.0.1:9050). Routes outbound HTTP through Tor")
	insecureTLS := flag.Bool("insecuretls", false, "skip TLS verification for outbound HTTP (dev only)")

	// Node-only
	nodeOnly := flag.Bool("nodeonly", false, "use local bitcoind only (no block explorer)")
	rpcURL := flag.String("rpcurl", "http://127.0.0.1:18332", "bitcoind RPC URL (testnet default)")
	rpcUser := flag.String("rpcuser", "", "bitcoind RPC username")
	rpcPass := flag.String("rpcpass", "", "bitcoind RPC password")

	// LND readiness (CLI + server)
	lnCheck := flag.Bool("lncheck", false, "also check LND readiness (cli mode)")
	lndEnabled := flag.Bool("lndenabled", false, "enable /lnready and LN in /report (server mode)")
	lndURL := flag.String("lndurl", "https://127.0.0.1:8080", "LND REST base URL")
	macaroonPath := flag.String("macaroon", "", "path to LND macaroon file (admin or readonly)")
	lndTLSInsecure := flag.Bool("lndinsecure", true, "skip TLS verify for LND (dev/self-signed)")

	// Server
	port := flag.String("port", "8080", "server port")

	flag.Parse()

	network := btc.Testnet
	if *networkStr == "mainnet" {
		network = btc.Mainnet
	}

	// Shared outbound HTTP client (optionally Tor-routed)
	httpClient, err := netx.NewHTTPClient(netx.ClientConfig{
		Timeout:       15 * time.Second,
		TorSocks5Addr: *torSocks,
		InsecureTLS:   *insecureTLS,
	})
	if err != nil {
		log.Fatalf("failed to build http client: %v", err)
	}

	switch *mode {
	case "cli":
		if *address == "" {
			fmt.Println("Usage:")
			fmt.Println("  go run . -mode=cli -address=<addr> -network=testnet")
			fmt.Println("Options:")
			fmt.Println("  -nodeonly=true -rpcurl=... -rpcuser=... -rpcpass=...")
			fmt.Println("  -tor=127.0.0.1:9050")
			fmt.Println("  -lncheck=true -lndurl=... -macaroon=/path/to.macaroon")
			os.Exit(1)
		}
		runCLI(httpClient, network, *address, *feeFallback, *feeLow, *nodeOnly, *rpcURL, *rpcUser, *rpcPass,
			*lnCheck, *lndURL, *macaroonPath, *lndTLSInsecure)
	case "server":
		runServer(httpClient, network, *feeFallback, *feeLow, *nodeOnly, *rpcURL, *rpcUser, *rpcPass,
			*port, *lndEnabled, *lndURL, *macaroonPath, *lndTLSInsecure)
	default:
		log.Fatalf("unknown mode: %s", *mode)
	}
}

func fetchOnChain(client *http.Client, network btc.Network, address string, feeFallback uint64,
	nodeOnly bool, rpcURL, rpcUser, rpcPass string,
) (score.Result, uint64, error) {
	feeRate := feeFallback
	mode := "explorer"

	var utxos []btc.UTXO
	var err error

	if nodeOnly {
		mode = "nodeonly"
		rpc := btc.NewBitcoindRPC(rpcURL, rpcUser, rpcPass, client)
		utxos, err = rpc.UTXOsForAddress(address)
		if err != nil {
			return score.Result{}, 0, err
		}
		if est, err := rpc.EstimateSmartFee(6); err == nil && est.FeeRateBTCPerKB > 0 {
			feeRate = planner.BTCPerKBToSatsPerVB(est.FeeRateBTCPerKB)
		}
	} else {
		utxos, err = btc.FetchUTXOsExplorer(client, address, network)
		if err != nil {
			return score.Result{}, 0, err
		}
	}

	res := score.Compute(score.Input{
		Address:      address,
		Network:      network,
		Mode:         mode,
		UTXOs:        utxos,
		FeeRateSatVB: feeRate,
	})
	return res, feeRate, nil
}

func runCLI(client *http.Client, network btc.Network, address string, feeFallback, feeLow uint64,
	nodeOnly bool, rpcURL, rpcUser, rpcPass string,
	lnCheck bool, lndURL, macaroonPath string, lndTLSInsecure bool,
) {
	onchain, feeRate, err := fetchOnChain(client, network, address, feeFallback, nodeOnly, rpcURL, rpcUser, rpcPass)
	if err != nil {
		log.Fatalf("onchain fetch failed: %v", err)
	}

	plan := planner.Decide(planner.Inputs{
		NumUTXOs:    onchain.NumUTXOs,
		DustCount:   onchain.DustUTXOs,
		FeeNowSatVB: feeRate,
		FeeLowSatVB: feeLow,
	})

	type Output struct {
		OnChain score.Result              `json:"onchain"`
		Plan    planner.ConsolidationPlan `json:"consolidation_plan"`
		LN      *ln.Readiness             `json:"ln_readiness,omitempty"`
	}

	out := Output{OnChain: onchain, Plan: plan}

	if lnCheck {
		if macaroonPath == "" {
			log.Println("lncheck requested but -macaroon is empty")
		} else {
			lndClient := &http.Client{Timeout: 10 * time.Second}
			c, err := ln.NewLNDClient(lndURL, macaroonPath, lndClient, lndTLSInsecure)
			if err != nil {
				log.Printf("lnd init error: %v", err)
			} else {
				info, err := c.GetInfo()
				if err != nil {
					log.Printf("lnd getinfo error: %v", err)
				} else {
					ready := ln.ComputeReadiness(info)
					out.LN = &ready
				}
			}
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func runServer(client *http.Client, network btc.Network, feeFallback, feeLow uint64,
	nodeOnly bool, rpcURL, rpcUser, rpcPass, port string,
	lndEnabled bool, lndURL, macaroonPath string, lndTLSInsecure bool,
) {
	cfg := api.Config{
		NodeOnly:        nodeOnly,
		Network:         network,
		FeeRateFallback: feeFallback,
		FeeLowSatVB:     feeLow,
		HTTPClient:      client,

		RPCURL:  rpcURL,
		RPCUser: rpcUser,
		RPCPass: rpcPass,

		LNDEnabled:     lndEnabled,
		LNDBaseURL:     lndURL,
		MacaroonPath:   macaroonPath,
		LNDClient:      &http.Client{Timeout: 10 * time.Second},
		LNDTLSInsecure: lndTLSInsecure,
	}

	s := api.NewServer(cfg)

	addr := ":" + port
	log.Printf("server listening on %s", addr)
	log.Printf("endpoints: /health, /check, /report, /lnready")
	log.Printf("example: /report?address=...&network=mainnet|testnet")
	if err := http.ListenAndServe(addr, s.Handler()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
