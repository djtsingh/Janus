# Janus

Janus blends user-friendly protection with technical rigor: a lightweight middleware that transparently blocks automated traffic by combining client-side Proof‑of‑Render (canvas) and optional Proof‑of‑Work, browser fingerprinting, TLS/JA3‑style checks, IP/Geo reputation and configurable heuristics — then issues short‑lived JWTs so real users pass seamlessly. For non‑technical teams it means fewer CAPTCHAs and better conversion; for engineers it’s easy to integrate, fully configurable (difficulty, weights, whitelists), and works offline (GeoIP/Redis optional) so you can tune UX vs. attacker cost without heavy infra.


## Features
- Browser fingerprint collection endpoint (`/janus/fingerprint`).
- Challenge issuance endpoint (`/janus/challenge`) and verification endpoint (`/janus/verify`).
- HTML/JS sensor (`assets/sensor.js`) and challenge page (`assets/challenge.html`).
- Rate limiting, GeoIP checks (using `GeoLite2-City.mmdb`), and JWT verification for short-lived access.

## Quickstart (development)

Prerequisites:

- Go 1.25 or newer
- `git`
- (Optional) Redis if you want to enable Redis-backed store features

Clone and build:

```powershell
Set-Location -Path "g:\2025\Janus"
go build ./cmd/janus
```

Run:

```powershell
.\janus.exe
# or
go run ./cmd/janus
```

By default the server expects `cert.pem` and `key.pem` in the working directory (self-signed certs are fine for local testing). It also expects `GeoLite2-City.mmdb` for geo lookups — if it's missing geo checks are skipped.

## Configuration

The service loads `config.yaml` if present. Run-time defaults are set in `internal/config/config.go`. Example values:

- `desktop_iterations`, `mobile_iterations` — iteration counts for PoR/PoW generation.
- `desktop_difficulty`, `mobile_difficulty` — number of leading zero bits required.
- `whitelist_ua`, `whitelist_ips` — bypass criteria.

You can create `config.yaml` at the repo root to override defaults.

## Endpoints
- `GET /` — Root (protected by Janus middleware)
- `GET /sensor.js` — Sensor JavaScript served to the client
- `POST /janus/fingerprint` — Accepts JSON fingerprint; expected by the challenge flow
- `GET /janus/challenge` — Returns a challenge for the requesting IP (must have earlier posted fingerprint)
- `POST /janus/verify` — Verifies proof and issues a `janus_token` cookie on success

## Development notes

- Main server entrypoint: `cmd/janus/main.go`.
- Middleware and routing: `internal/middleware/janus.go`.
- Challenge generation & verification: `internal/challenge/challenge.go`.
- Fingerprint types: `internal/types/types.go`.

If you change code, run `go build ./...` to verify compilation.

## CI / Workflow

A GitHub Actions workflow (in `.github/workflows/ci.yml`) is included to run `go vet` and `go build ./...` on push and pull requests.

## License

This project is licensed under the terms described in the `LICENSE` file.