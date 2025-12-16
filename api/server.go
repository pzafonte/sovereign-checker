package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"sovereign-checker/btc"
	"sovereign-checker/ln"
	"sovereign-checker/planner"
	"sovereign-checker/score"
)

type Config struct {
	// On-chain
	NodeOnly        bool
	Network         btc.Network
	FeeRateFallback uint64
	FeeLowSatVB     uint64

	// Shared HTTP client (may be Tor-routed)
	HTTPClient *http.Client

	// bitcoind (optional)
	RPCURL  string
	RPCUser string
	RPCPass string

	// LND (optional)
	LNDEnabled     bool
	LNDBaseURL     string
	MacaroonPath   string
	LNDClient      *http.Client
	LNDTLSInsecure bool
}

type Server struct {
	cfg Config
}

func NewServer(cfg Config) *Server { return &Server{cfg: cfg} }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/check", s.handleCheck)
	mux.HandleFunc("/lnready", s.handleLNReady)
	mux.HandleFunc("/report", s.handleReport) // NEW
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) resolveNetworkFromQuery(r *http.Request) btc.Network {
	qn := r.URL.Query().Get("network")
	if qn == "mainnet" {
		return btc.Mainnet
	}
	if qn == "testnet" {
		return btc.Testnet
	}
	return s.cfg.Network
}

func (s *Server) fetchUTXOs(address string, network btc.Network) ([]btc.UTXO, uint64, string, error) {
	mode := "explorer"
	feeRate := s.cfg.FeeRateFallback

	if s.cfg.NodeOnly {
		mode = "nodeonly"
		rpc := btc.NewBitcoindRPC(s.cfg.RPCURL, s.cfg.RPCUser, s.cfg.RPCPass, s.cfg.HTTPClient)
		utxos, err := rpc.UTXOsForAddress(address)
		if err != nil {
			return nil, 0, mode, err
		}
		if est, err := rpc.EstimateSmartFee(6); err == nil && est.FeeRateBTCPerKB > 0 {
			feeRate = planner.BTCPerKBToSatsPerVB(est.FeeRateBTCPerKB)
		}
		return utxos, feeRate, mode, nil
	}

	utxos, err := btc.FetchUTXOsExplorer(s.cfg.HTTPClient, address, network)
	return utxos, feeRate, mode, err
}

func (s *Server) maybeLNReadiness() *ln.Readiness {
	if !s.cfg.LNDEnabled || s.cfg.MacaroonPath == "" || s.cfg.LNDBaseURL == "" {
		return nil
	}
	c, err := ln.NewLNDClient(s.cfg.LNDBaseURL, s.cfg.MacaroonPath, s.cfg.LNDClient, s.cfg.LNDTLSInsecure)
	if err != nil {
		log.Printf("lnd init error (omitting): %v", err)
		return nil
	}
	info, err := c.GetInfo()
	if err != nil {
		log.Printf("lnd getinfo error (omitting): %v", err)
		return nil
	}
	ready := ln.ComputeReadiness(info)
	return &ready
}

func sovereigntySummary(onchain score.Result, plan planner.ConsolidationPlan, lnReady *ln.Readiness) string {
	planPart := "Plan: WAIT"
	if plan.Recommended {
		planPart = "Plan: CONSOLIDATE"
	}

	lnPart := "LN: n/a"
	if lnReady != nil {
		if lnReady.Ready {
			lnPart = fmt.Sprintf("LN: READY (%d/100)", lnReady.Score)
		} else {
			lnPart = fmt.Sprintf("LN: NOT READY (%d/100)", lnReady.Score)
		}
	}

	return fmt.Sprintf(
		"Score %d/100 • %d UTXOs (%d dust) • Fee %d sat/vB • %s • %s",
		onchain.SovereigntyScore,
		onchain.NumUTXOs,
		onchain.DustUTXOs,
		onchain.FeeRateSatVB,
		planPart,
		lnPart,
	)
}

func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	addr := r.URL.Query().Get("address")
	if addr == "" {
		http.Error(w, "missing address", http.StatusBadRequest)
		return
	}

	network := s.resolveNetworkFromQuery(r)

	utxos, feeRate, mode, err := s.fetchUTXOs(addr, network)
	if err != nil {
		log.Printf("fetch utxos error: %v", err)
		http.Error(w, "failed to fetch utxos", http.StatusBadGateway)
		return
	}

	onchain := score.Compute(score.Input{
		Address:      addr,
		Network:      network,
		Mode:         mode,
		UTXOs:        utxos,
		FeeRateSatVB: feeRate,
	})

	plan := planner.Decide(planner.Inputs{
		NumUTXOs:    onchain.NumUTXOs,
		DustCount:   onchain.DustUTXOs,
		FeeNowSatVB: feeRate,
		FeeLowSatVB: s.cfg.FeeLowSatVB,
	})

	type Response struct {
		OnChain score.Result              `json:"onchain"`
		Plan    planner.ConsolidationPlan `json:"consolidation_plan"`
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(Response{OnChain: onchain, Plan: plan})
}

func (s *Server) handleLNReady(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.LNDEnabled {
		http.Error(w, "lnd not enabled", http.StatusBadRequest)
		return
	}
	ready := s.maybeLNReadiness()
	if ready == nil {
		http.Error(w, "lnd not configured or unavailable", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ready)
}

// NEW: /report combines on-chain + plan + optional LN + a judge-friendly summary
func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	addr := r.URL.Query().Get("address")
	if addr == "" {
		http.Error(w, "missing address", http.StatusBadRequest)
		return
	}
	network := s.resolveNetworkFromQuery(r)

	utxos, feeRate, mode, err := s.fetchUTXOs(addr, network)
	if err != nil {
		log.Printf("fetch utxos error: %v", err)
		http.Error(w, "failed to fetch utxos", http.StatusBadGateway)
		return
	}

	onchain := score.Compute(score.Input{
		Address:      addr,
		Network:      network,
		Mode:         mode,
		UTXOs:        utxos,
		FeeRateSatVB: feeRate,
	})

	plan := planner.Decide(planner.Inputs{
		NumUTXOs:    onchain.NumUTXOs,
		DustCount:   onchain.DustUTXOs,
		FeeNowSatVB: feeRate,
		FeeLowSatVB: s.cfg.FeeLowSatVB,
	})

	lnReady := s.maybeLNReadiness()

	type Report struct {
		SovereigntySummary string                    `json:"sovereignty_summary"`
		OnChain            score.Result              `json:"onchain"`
		Plan               planner.ConsolidationPlan `json:"consolidation_plan"`
		LN                 *ln.Readiness             `json:"ln_readiness,omitempty"`
	}

	report := Report{
		SovereigntySummary: sovereigntySummary(onchain, plan, lnReady),
		OnChain:            onchain,
		Plan:               plan,
		LN:                 lnReady,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(report)
}
