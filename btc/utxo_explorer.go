package btc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Network string

const (
	Mainnet Network = "mainnet"
	Testnet Network = "testnet"
)

type blockstreamUTXO struct {
	TxID   string `json:"txid"`
	Vout   int    `json:"vout"`
	Value  uint64 `json:"value"`
	Status struct {
		Confirmed   bool `json:"confirmed"`
		BlockHeight int  `json:"block_height"`
	} `json:"status"`
}

type UTXO struct {
	TxID        string `json:"txid"`
	Vout        int    `json:"vout"`
	ValueSats   uint64 `json:"value_sats"`
	Confirmed   bool   `json:"confirmed"`
	BlockHeight int    `json:"block_height"`
	Source      string `json:"source"` // "explorer" or "bitcoind"
}

func explorerBaseURL(network Network) (string, error) {
	switch network {
	case Mainnet:
		return "https://blockstream.info/api", nil
	case Testnet:
		return "https://blockstream.info/testnet/api", nil
	default:
		return "", fmt.Errorf("unsupported network: %s", network)
	}
}

func FetchUTXOsExplorer(client *http.Client, address string, network Network) ([]UTXO, error) {
	base, err := explorerBaseURL(network)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/address/%s/utxo", base, address)
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch utxos (explorer): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("explorer status %d: %s", resp.StatusCode, string(body))
	}

	var raw []blockstreamUTXO
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode utxos: %w", err)
	}

	out := make([]UTXO, 0, len(raw))
	for _, r := range raw {
		out = append(out, UTXO{
			TxID:        r.TxID,
			Vout:        r.Vout,
			ValueSats:   r.Value,
			Confirmed:   r.Status.Confirmed,
			BlockHeight: r.Status.BlockHeight,
			Source:      "explorer",
		})
	}
	return out, nil
}

// Rough fee estimate for sweep cost (not production):
// size ≈ inputs*148 + outputs*34 + 10 (vbytes-ish) * feerate (sats/vB)
func EstimateSweepFee(numInputs, numOutputs int, feeRateSatsPerVByte uint64) uint64 {
	if numInputs <= 0 || numOutputs <= 0 {
		return 0
	}
	size := uint64(numInputs*148 + numOutputs*34 + 10)
	return size * feeRateSatsPerVByte
}
