package score

import "sovereign-checker/btc"

type Result struct {
	Address           string     `json:"address"`
	Network           string     `json:"network"`
	Mode              string     `json:"mode"` // "explorer" or "nodeonly"
	TotalBalanceSats  uint64     `json:"total_balance_sats"`
	NumUTXOs          int        `json:"num_utxos"`
	DustUTXOs         int        `json:"dust_utxos"`
	EstimatedSweepFee uint64     `json:"estimated_sweep_fee_sats"`
	FeeRateSatVB      uint64     `json:"fee_rate_sat_vb"`
	SovereigntyScore  int        `json:"sovereignty_score"`
	Warnings          []string   `json:"warnings"`
	Notes             []string   `json:"notes"`
	UTXOs             []btc.UTXO `json:"utxos"`
}

type Input struct {
	Address      string
	Network      btc.Network
	Mode         string
	UTXOs        []btc.UTXO
	FeeRateSatVB uint64
}

func CountDust(utxos []btc.UTXO, dustThreshold uint64) int {
	n := 0
	for _, u := range utxos {
		if u.ValueSats < dustThreshold {
			n++
		}
	}
	return n
}

func Compute(in Input) Result {
	var total uint64
	for _, u := range in.UTXOs {
		total += u.ValueSats
	}
	nUTXOs := len(in.UTXOs)
	dust := CountDust(in.UTXOs, 1000)

	estimatedFee := btc.EstimateSweepFee(nUTXOs, 1, in.FeeRateSatVB)

	score := 50
	warnings := []string{}
	notes := []string{}

	switch {
	case nUTXOs == 0:
		score -= 10
		warnings = append(warnings, "No UTXOs found for this address.")
	case nUTXOs > 50:
		score -= 20
		warnings = append(warnings, "Very high UTXO count; sweeping could be expensive.")
	case nUTXOs > 10:
		score -= 10
		warnings = append(warnings, "Moderate UTXO count; consider consolidation when fees are low.")
	default:
		score += 10
		notes = append(notes, "UTXO count looks reasonable.")
	}

	if dust > 0 {
		score -= 10
		warnings = append(warnings, "Dust UTXOs (< 1000 sats) detected; may be uneconomical to spend.")
	}

	if estimatedFee > 0 && total > 0 {
		ratio := float64(estimatedFee) / float64(total)
		if ratio > 0.05 {
			score -= 10
			warnings = append(warnings, "Estimated sweep fee is >5% of total balance.")
		} else {
			score += 5
			notes = append(notes, "Sweep fee looks small relative to balance.")
		}
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	notes = append(notes,
		"Use fresh addresses for incoming payments to reduce address reuse.",
		"Be careful: consolidation can reduce privacy by linking UTXOs.",
	)

	return Result{
		Address:           in.Address,
		Network:           string(in.Network),
		Mode:              in.Mode,
		TotalBalanceSats:  total,
		NumUTXOs:          nUTXOs,
		DustUTXOs:         dust,
		EstimatedSweepFee: estimatedFee,
		FeeRateSatVB:      in.FeeRateSatVB,
		SovereigntyScore:  score,
		Warnings:          warnings,
		Notes:             notes,
		UTXOs:             in.UTXOs,
	}
}
