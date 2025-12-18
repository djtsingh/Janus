## Janus 🚪🛡️

Janus is a lightweight Go middleware designed to stop automated bots while preserving a smooth experience for real users. It combines client-side Proof‑of‑Render (PoR) and optional Proof‑of‑Work (PoW), browser fingerprinting, TLS-like fingerprint heuristics, IP/Geo reputation, and configurable scoring to decide whether to challenge a visitor — then issues short-lived JWT cookies so verified users pass through seamlessly.

Why Janus? ✨
- Less friction than traditional CAPTCHAs — challenges are small, transparent, and often invisible to real users.
- Multi-signal detection — not a single noisy heuristic but a weighted score from headers, fingerprint, TLS/JA3-like info, geo/ip reputation, and rate limits.
- Tunable — you control difficulty, weights, and whitelists in `config.yaml`.

Quick USP blurb
> Janus blends usability and security: fewer CAPTCHAs for users, stronger bot resistance for your site — easy to integrate and fully configurable. 🚀

## 🚀 Quickstart (development)

Prerequisites
- Go 1.25+
- Optional: `openssl` (for self-signed certs) and Redis (for production store)

Clone, build and run (PowerShell):
```powershell
Set-Location -Path "g:\2025\Janus"
go build ./cmd/janus
.\janus.exe
# or
go run ./cmd/janus
```

Notes
- The server expects `cert.pem` and `key.pem` in the working directory for HTTPS in `cmd/janus/main.go`. For local testing, generate self-signed certs with `openssl`.
- Drop `GeoLite2-City.mmdb` in the repo root to enable geo-based checks (optional).

## 🧭 What Janus protects (high-level flow)
1. A visitor requests a protected page — `JanusMiddleware` intercepts every request.
2. Quick checks: if request is for Janus API (`/janus/*`) or sensor, serve it; if visitor has a valid `janus_token` cookie, allow through.
3. If unverified, the middleware serves a challenge page (`assets/challenge.html`) which loads `assets/sensor.js`.
4. The browser posts a fingerprint to `POST /janus/fingerprint` and requests `GET /janus/challenge`.
5. Server issues a tiny challenge (nonce, seed, iterations, difficulty).
6. Client computes a proof (PoR uses a canvas hash; PoW performs light hashing) and posts to `POST /janus/verify`.
7. Server verifies: nonce/seed/IP/timestamp/iterations/canvas-hash and required leading zero bits in SHA256(proof).
8. On success, server sets a `janus_token` JWT cookie; future requests pass without challenge.

## 🔍 Endpoints
- `POST /janus/fingerprint` — store client fingerprint (JSON).
- `GET /janus/challenge` — retrieve a challenge for the requesting IP.
- `POST /janus/verify` — submit proof; server validates and issues `janus_token` on success.
- `GET /sensor.js` — client-side sensor script.

## 🧪 Quick local test (shortcut)
1. Create a `config.yaml` that sets `desktop_difficulty: 0` and `mobile_difficulty: 0` to skip actual PoW while testing.
2. Start the server:
```powershell
go run ./cmd/janus
```
3. Use a browser to visit `https://localhost:8080/` (accept self-signed cert) and follow the challenge flow.
4. Or simulate with curl (example):
```bash
curl -k -X POST https://localhost:8080/janus/fingerprint \
	-H "Content-Type: application/json" \
	-d '{"client_ip":"127.0.0.1","canvas_hash":"test","isMobile":false}'

curl -k https://localhost:8080/janus/challenge

# use nonce/seed from response to craft a proof with difficulty 0 and POST to /janus/verify
```

## 🛠️ For developers
- Entrypoint: [cmd/janus/main.go](cmd/janus/main.go)
- Middleware: [internal/middleware/janus.go](internal/middleware/janus.go)
- Challenge logic: [internal/challenge/challenge.go](internal/challenge/challenge.go)
- Fingerprint types: [internal/types/types.go](internal/types/types.go)
- Handlers: [internal/handlers/handlers.go](internal/handlers/handlers.go)

Run basic checks locally:
```powershell
go mod download
go vet ./...
go build ./...
```

## 🤝 Contributing & Code of Conduct
We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for how to open issues, propose changes, run tests, and follow our code style. All contributors must follow the Code of Conduct in [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

## 🧭 Roadmap & Creative ideas
- Dashboard for live-suspicion scoring and metrics 📊
- Pluggable storage backends (Redis, Postgres) for scaling challenges 🗄️
- Prometheus metrics + Grafana dashboards 📈
- Optional WebSocket-based real-time challenge status channel ⚡

## ❤️ Sponsor / Use cases
- Great for sites that need lower-friction bot defense than CAPTCHA (login forms, comment systems, scrapers), or as a second layer in a defense-in-depth strategy.

## License
See [LICENSE](LICENSE) — permissive.