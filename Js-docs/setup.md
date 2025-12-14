# Setup & Quickstart

1. Prerequisites
   - Go 1.25+
   - Optional: `openssl` for dev TLS certs

2. Generate test certs (dev):
```powershell
openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem -days 365 -nodes -subj "/CN=localhost"
```

3. (Optional) Copy `config.example.yaml` to `config.yaml` and tweak values for local testing.

4. Build and run:
```powershell
go mod download
go run ./cmd/janus
```

5. Visit `https://localhost:8080/` and follow the challenge flow.
