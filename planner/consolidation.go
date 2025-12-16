package planner

import "math"

type ConsolidationPlan struct {
	Recommended     bool     `json:"recommended"`
	Reason          string   `json:"reason"`
	SuggestedTarget string   `json:"suggested_target"`
	Notes           []string `json:"notes"`
	FeeNowSatVB     uint64   `json:"fee_now_sat_vb"`
	FeeLowSatVB     uint64   `json:"fee_low_sat_vb"`
}

func BTCPerKBToSatsPerVB(btcPerKB float64) uint64 {
	satsPerKB := btcPerKB * 100_000_000.0
	satsPerVB := satsPerKB / 1000.0
	if satsPerVB < 1 {
		return 1
	}
	return uint64(math.Round(satsPerVB))
}

type Inputs struct {
	NumUTXOs    int
	DustCount   int
	FeeNowSatVB uint64
	FeeLowSatVB uint64
}

func Decide(in Inputs) ConsolidationPlan {
	notes := []string{}

	if in.NumUTXOs <= 1 {
		return ConsolidationPlan{
			Recommended:     false,
			Reason:          "Already 0–1 UTXOs; consolidation not needed.",
			SuggestedTarget: "N/A",
			Notes:           notes,
			FeeNowSatVB:     in.FeeNowSatVB,
			FeeLowSatVB:     in.FeeLowSatVB,
		}
	}

	pressure := 0
	if in.NumUTXOs > 10 {
		pressure++
	}
	if in.NumUTXOs > 30 {
		pressure++
	}
	if in.DustCount > 0 {
		pressure++
	}

	if pressure == 0 {
		return ConsolidationPlan{
			Recommended:     false,
			Reason:          "UTXO set looks manageable; consolidation optional.",
			SuggestedTarget: "N/A",
			Notes: append(notes,
				"Consolidate only when fees are low if you want to simplify future spending.",
			),
			FeeNowSatVB: in.FeeNowSatVB,
			FeeLowSatVB: in.FeeLowSatVB,
		}
	}

	if in.FeeNowSatVB <= in.FeeLowSatVB {
		return ConsolidationPlan{
			Recommended:     true,
			Reason:          "Fees look low and UTXO fragmentation is high; consolidate now to reduce future fees.",
			SuggestedTarget: "Consolidate to 1–3 UTXOs",
			Notes: append(notes,
				"Consolidation can reduce privacy by linking coins.",
				"Consider privacy tools before consolidating large amounts.",
			),
			FeeNowSatVB: in.FeeNowSatVB,
			FeeLowSatVB: in.FeeLowSatVB,
		}
	}

	return ConsolidationPlan{
		Recommended:     false,
		Reason:          "Fees look elevated; waiting for cheaper fees is likely better unless you must move coins soon.",
		SuggestedTarget: "Wait for a low-fee window",
		Notes: append(notes,
			"If you must spend soon, consider consolidating only the smallest UTXOs.",
			"Re-check fee estimates from your node periodically.",
		),
		FeeNowSatVB: in.FeeNowSatVB,
		FeeLowSatVB: in.FeeLowSatVB,
	}
}
