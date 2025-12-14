# Contributing to Janus 🎉

Thank you for considering contributing to Janus! We welcome bug reports, feature requests, documentation improvements, and code contributions. This file explains how to get started and the workflow we prefer for contributions.

## Table of contents
- How to contribute
- Development workflow
- Coding style
- Running tests & checks
- Submitting a pull request
- Communication & support

## How to contribute

1. Search existing issues to see if your idea or bug is already reported.
2. If nothing exists, open an issue describing the problem or feature, including steps to reproduce and expected behavior.
3. If you want to write code, comment on the issue that you plan to work on it or pick up an issue labeled `good first issue`.

## Development workflow

1. Fork the repo and create a branch from `main`:
```bash
git checkout -b feat/short-description
```
2. Keep changes focused and small.
3. Commit with clear messages (conventional style encouraged):
```
feat: add challenge timeout
fix: correct IP parsing in middleware
docs: update README example
```

## Coding style

- Format Go code with `gofmt` and `goimports` before committing.
- Keep functions small and focused.
- Add comments for exported functions and types.

## Running checks locally

Before opening a PR, run the following locally:
```bash
go mod download
go vet ./...
go test ./...    # if tests exist
go build ./...
```

## Tests
- Add unit tests for new features when reasonable.
- Tests help a lot — they make PRs easier to review and merge.

## Submitting a pull request

1. Push your branch to your fork and open a Pull Request against `main`.
2. In the PR description, include a summary of changes, why they are needed, and any migration notes.
3. Maintain backwards compatibility where possible; call out breaking changes clearly.
4. Maintainers will review and may suggest changes. Respond to review comments promptly.

## Communication & support

- For questions, open an issue or a discussion in the repo.
- Be respectful and follow the [Code of Conduct](CODE_OF_CONDUCT.md).

Thanks again — we look forward to your contributions! 🚀
