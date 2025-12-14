# Architecture

Janus is structured to be lightweight and modular:

- `cmd/janus` — application entrypoint and server wiring.
- `internal/middleware` — primary HTTP middleware that enforces challenge flow, scoring and routing.
- `internal/challenge` — generation and verification logic for PoR/PoW.
- `internal/handlers` — HTTP handlers (e.g., fingerprint receiver).
- `internal/store` — optional Redis-backed session/nonce store.
- `assets/` — static JS/HTML for client sensor and challenge UI.

Data flow
1. Client request -> `JanusMiddleware`.
2. If unverified -> serve `challenge.html` which runs `sensor.js`.
3. Client posts fingerprint -> `POST /janus/fingerprint`.
4. Client requests `GET /janus/challenge` -> server issues challenge.
5. Client posts proof to `POST /janus/verify` -> server verifies and issues `janus_token` cookie.
