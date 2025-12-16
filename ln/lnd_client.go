package ln

import (
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type LNDClient struct {
	BaseURL string
	MacHex  string
	Client  *http.Client
}

func NewLNDClient(baseURL, macaroonPath string, client *http.Client, tlsInsecure bool) (*LNDClient, error) {
	macBytes, err := os.ReadFile(macaroonPath)
	if err != nil {
		return nil, err
	}
	macHex := hex.EncodeToString(macBytes)

	if client == nil {
		tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: tlsInsecure}} // dev only
		client = &http.Client{Timeout: 10 * time.Second, Transport: tr}
	}

	return &LNDClient{BaseURL: baseURL, MacHex: macHex, Client: client}, nil
}

func (c *LNDClient) get(path string, out interface{}) error {
	req, err := http.NewRequest("GET", c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Grpc-Metadata-macaroon", c.MacHex)

	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("lnd http %d: %s", resp.StatusCode, string(b))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

type GetInfoResponse struct {
	IdentityPubkey    string `json:"identity_pubkey"`
	Alias             string `json:"alias"`
	BlockHeight       int    `json:"block_height"`
	Version           string `json:"version"`
	NumActiveChannels int    `json:"num_active_channels"`
	NumPeers          int    `json:"num_peers"`
	SyncedToChain     bool   `json:"synced_to_chain"`
	SyncedToGraph     bool   `json:"synced_to_graph"`
}

func (c *LNDClient) GetInfo() (GetInfoResponse, error) {
	var res GetInfoResponse
	err := c.get("/v1/getinfo", &res)
	return res, err
}

type Readiness struct {
	Ready   bool            `json:"ready"`
	Score   int             `json:"score"`
	Reasons []string        `json:"reasons"`
	Info    GetInfoResponse `json:"info"`
}

func ComputeReadiness(info GetInfoResponse) Readiness {
	score := 50
	reasons := []string{}

	if info.SyncedToChain {
		score += 25
	} else {
		score -= 25
		reasons = append(reasons, "LND not synced to chain")
	}

	if info.NumPeers > 0 {
		score += 10
	} else {
		score -= 10
		reasons = append(reasons, "No peers connected")
	}

	if info.NumActiveChannels > 0 {
		score += 15
	} else {
		reasons = append(reasons, "No active channels (opening a channel required for most outgoing LN payments)")
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return Readiness{
		Ready:   info.SyncedToChain && info.NumPeers > 0,
		Score:   score,
		Reasons: reasons,
		Info:    info,
	}
}
