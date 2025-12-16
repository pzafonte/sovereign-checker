package btc

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type BitcoindRPC struct {
	URL      string
	User     string
	Password string
	Client   *http.Client
}

func NewBitcoindRPC(url, user, pass string, client *http.Client) *BitcoindRPC {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &BitcoindRPC{URL: url, User: user, Password: pass, Client: client}
}

type rpcReq struct {
	Jsonrpc string        `json:"jsonrpc"`
	ID      string        `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type rpcResp struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	ID string `json:"id"`
}

func (r *BitcoindRPC) call(method string, params ...interface{}) (json.RawMessage, error) {
	body, _ := json.Marshal(rpcReq{
		Jsonrpc: "1.0",
		ID:      "sovereign-checker",
		Method:  method,
		Params:  params,
	})

	req, err := http.NewRequest("POST", r.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(r.User + ":" + r.Password))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out rpcResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.Error != nil {
		return nil, fmt.Errorf("bitcoind rpc error %d: %s", out.Error.Code, out.Error.Message)
	}
	return out.Result, nil
}

type ListUnspentItem struct {
	TxID          string  `json:"txid"`
	Vout          int     `json:"vout"`
	Address       string  `json:"address"`
	AmountBTC     float64 `json:"amount"`
	Confirmations int     `json:"confirmations"`
	Spendable     bool    `json:"spendable"`
	Solvable      bool    `json:"solvable"`
}

func (r *BitcoindRPC) ListUnspent(minConf, maxConf int, addresses []string) ([]ListUnspentItem, error) {
	raw, err := r.call("listunspent", minConf, maxConf, addresses)
	if err != nil {
		return nil, err
	}
	var items []ListUnspentItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	return items, nil
}

type EstimateSmartFeeResult struct {
	FeeRateBTCPerKB float64  `json:"feerate"`
	Blocks          int      `json:"blocks"`
	Errors          []string `json:"errors"`
}

func (r *BitcoindRPC) EstimateSmartFee(confTarget int) (EstimateSmartFeeResult, error) {
	raw, err := r.call("estimatesmartfee", confTarget)
	if err != nil {
		return EstimateSmartFeeResult{}, err
	}
	var res EstimateSmartFeeResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return EstimateSmartFeeResult{}, err
	}
	return res, nil
}

func btcToSats(b float64) uint64 {
	return uint64(b * 100_000_000.0) // hackathon OK; float caveat
}

func (r *BitcoindRPC) UTXOsForAddress(address string) ([]UTXO, error) {
	items, err := r.ListUnspent(0, 9999999, []string{address})
	if err != nil {
		return nil, err
	}
	out := make([]UTXO, 0, len(items))
	for _, it := range items {
		out = append(out, UTXO{
			TxID:        it.TxID,
			Vout:        it.Vout,
			ValueSats:   btcToSats(it.AmountBTC),
			Confirmed:   it.Confirmations > 0,
			BlockHeight: 0,
			Source:      "bitcoind",
		})
	}
	return out, nil
}
