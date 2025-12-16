# Bitcoin Wallet Sovereignty Health Checker

A node-first tool that generates a **single JSON report** describing a Bitcoin wallet’s
UTXO structure, fee exposure, consolidation guidance, and optional Lightning Network
readiness.

The goal is to make **wallet-level sovereignty risks explicit** using protocol-native data.

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

## What the tool does

Given a Bitcoin address, the tool produces a **single JSON report** that includes:

- UTXO count, total value, and dust detection
- Estimated sweep / consolidation cost using current fee rates
- Deterministic guidance: `WAIT`, `CONSOLIDATE`, or `CONSOLIDATE_WITH_CAUTION`
- Optional Lightning Network readiness (via LND)
- A one-line **sovereignty summary** suitable for logs or dashboards

The tool prefers **node-derived data** and clearly labels any fallback to third-party
explorers.

---

## Sovereignty model

This project uses a **practical definition of sovereignty**:

- Prefer local node truth over explorer opinions
- Reduce hidden dependencies on wallet defaults
- Surface fee and spendability constraints explicitly
- Preserve user control by exposing tradeoffs, not automating decisions

Explorer-backed mode exists as a **transparent fallback**, not a source of truth.

---

## Architecture overview

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

## Example output

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

## Command-line examples

Below are common usage patterns, from minimal inspection to full sovereignty-focused configurations.

---

### Basic CLI usage

Generate a report for a Bitcoin address on testnet:

```bash
go run . \
  -mode=cli \
  -network=testnet \
  -address=tb1qexampleaddress

