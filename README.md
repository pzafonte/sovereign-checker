
# Bitcoin Wallet Sovereignty Health Checker

A node-first tool that generates a **single JSON report** describing a Bitcoin wallet’s
UTXO structure, fee exposure, consolidation guidance, and optional Lightning Network
readiness.

Built for **bitcoin++ Taipei**.

---

## Motivation

Bitcoin wallets can become expensive or impractical to use over time due to:

- UTXO fragmentation
- Dust accumulation
- Poor consolidation timing
- High fee environments
- Missing or unavailable Lightning fallback

Most wallets abstract these details away.

This project answers a concrete question:

> **Is this Bitcoin wallet cheap, expensive, or risky to use right now — and why?**

---

## What the Tool Does

Given a Bitcoin address, the tool produces a **single JSON report** that includes:

- UTXO count, total value, and dust detection
- Estimated sweep / consolidation cost using current fee rates
- Deterministic guidance: `WAIT`, `CONSOLIDATE`, or `CONSOLIDATE_WITH_CAUTION`
- Optional Lightning Network readiness (via LND)
- A one-line **sovereignty summary** suitable for logs or dashboards

The tool prefers **local node data** and explicitly labels any fallback to third-party
explorers.

---

## Sovereignty Model

This project uses a **practical definition of sovereignty**:

- Prefer local node truth over explorer opinions
- Reduce hidden dependencies on wallet defaults
- Surface fee and spendability constraints explicitly
- Preserve user control by exposing tradeoffs, not automating decisions

Explorer-backed mode exists as a **transparent fallback**, not a source of truth.

---

## What This Tool Does NOT Do

* Does not move funds
* Does not create transactions
* Does not auto-consolidate
* Does not cluster addresses

It surfaces **constraints**, not prescriptions.

---

## Why This Matters

If you do not understand:

* how many UTXOs you have,
* how much it costs to move them,
* and when consolidation becomes necessary,

then your ability to use Bitcoin is implicitly delegated to wallet defaults and timing
you do not control.

This tool makes those tradeoffs explicit.

---

## Architecture Overview

- **Language:** Go
- **On-chain data**
  - Bitcoin Core RPC (node-only mode)
  - Block explorer fallback (address-only analysis)
- **Networking**
  - Optional Tor routing for outbound requests
- **Lightning**
  - LND REST API (`/v1/getinfo`)
  - Read-only macaroon authentication
- **Interfaces**
  - CLI
  - HTTP server with `/report` endpoint
- **Design**
  - Deterministic analysis
  - Explicit data provenance
  - No transaction creation or fund movement

---

## Installation Requirements

### Required
- **Go ≥ 1.21**

### Optional (Recommended)
- **Bitcoin Core** (node-only mode)
- **Tor** (privacy-preserving queries)
- **LND** (Lightning readiness checks)

---

## Dependency Setup

### Install Go (macOS)

```bash
brew install go
go version
````

---

### Install Bitcoin Core (Optional)

```bash
brew install bitcoin
bitcoind -testnet -daemon
bitcoin-cli -testnet getblockchaininfo
```

---

### Install Tor (Optional but Recommended)

```bash
brew install tor
brew services start tor
nc -zv 127.0.0.1 9050
```

---

### Install LND (Optional)

```TBD
```

---

## Build & Run

```bash
git clone git@github.com:pzafonte/sovereign-checker.git
cd sovereign-checker
go build
```

---

## Example Report Output (Hypothetical)

```json
{
  "address": "tb1qexample...",
  "network": "testnet",
  "data_source": "explorer_fallback_over_tor",
  "onchain": {
    "utxos": {
      "count": 2,
      "dust_count": 1,
      "total_sats": 106804
    }
  },
  "fees": {
    "feerate_sat_vb": 6,
    "estimated_sweep_sats": 2160
  },
  "plan": {
    "action": "CONSOLIDATE_WITH_CAUTION"
  },
  "sovereignty_summary": "Small UTXOs increase fee sensitivity; consolidation improves spendability but has privacy tradeoffs."
}
```

---

## Command-Line Usage

### Basic CLI

```bash
go run . \
  -mode=cli \
  -network=testnet \
  -address=tb1qexampleaddress
```

---

### CLI with Tor Routing

```bash
go run . \
  -mode=cli \
  -tor=127.0.0.1:9050 \
  -network=testnet \
  -address=tb1qexampleaddress
```

---

### CLI Using Bitcoin Core (Node-Only)

```bash
go run . \
  -mode=cli \
  -network=testnet \
  -address=tb1qexampleaddress \
  -bitcoindrpc=127.0.0.1:18332 \
  -rpcuser=demo \
  -rpcpassword=demopass
```

---

### CLI with Lightning Readiness

```bash
go run . \
  -mode=cli \
  -network=testnet \
  -address=tb1qexampleaddress \
  -lncheck=true \
  -lndurl=https://127.0.0.1:8080 \
  -macaroon="$HOME/Library/Application Support/Lnd/data/chain/bitcoin/testnet/readonly.macaroon" \
  -lndinsecure=true
```

---

### Full Sovereignty Configuration (Tor + Lightning)

```bash
go run . \
  -mode=cli \
  -tor=127.0.0.1:9050 \
  -network=testnet \
  -address=tb1qexampleaddress \
  -lncheck=true \
  -lndurl=https://127.0.0.1:8080 \
  -macaroon="$HOME/Library/Application Support/Lnd/data/chain/bitcoin/testnet/readonly.macaroon" \
  -lndinsecure=true
```

---

## HTTP Server Mode

```bash
go run . -mode=server -port=8080
curl "http://localhost:8080/report?address=tb1qexampleaddress&network=testnet"
```

---

## HTTP Server with Lightning Enabled

```bash
go run . \
  -mode=server \
  -port=8080 \
  -lndenabled=true \
  -lndurl=https://127.0.0.1:8080 \
  -macaroon="$HOME/Library/Application Support/Lnd/data/chain/bitcoin/testnet/readonly.macaroon" \
  -lndinsecure=true

curl "http://localhost:8080/report?address=tb1qexampleaddress&network=testnet"
```

---

## Demo-Friendly Output Filtering

```bash
go run . -mode=cli -network=testnet -address=tb1qexampleaddress | jq '.sovereignty_summary'
go run . -mode=cli -network=testnet -address=tb1qexampleaddress | jq '.plan'
```

---

## Minimal Judge Demo Command

```bash
go run . \
  -mode=cli \
  -tor=127.0.0.1:9050 \
  -network=testnet \
  -address=tb1qexampleaddress
```


