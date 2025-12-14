# Development Guide

1. Branching
   - Create feature branches from `main`: `git checkout -b feat/awesome`

2. Formatting & static checks
```bash
gofmt -w .
go vet ./...
go build ./...
```

3. Running tests
- Add unit tests under `*_test.go` files; run `go test ./...`.

4. Making changes
- Keep commits small and descriptive.
- Update `CHANGELOG.md` (if present) for noteworthy changes.
