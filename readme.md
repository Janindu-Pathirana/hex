# hex

A CLI tool for analyzing companies on the Sri Lanka stock market. Installable via Homebrew.

## Prerequisites

- [Go](https://go.dev/dl/) 1.26 or later

## Build

From the project root:

```bash
go build -o hex .
```

Build with a release version:

```bash
go build -ldflags "-X github.com/Janindu-Pathirana/hex/cmd.Version=0.1.0" -o hex .
```

## Run

Run without installing:

```bash
go run .
```

Or use the built binary:

```bash
./hex
```

Show help:

```bash
./hex --help
```

Check the version:

```bash
./hex version
```

Install to your `$GOPATH/bin` (or `$GOBIN`):

```bash
go install .
hex version
```

## Homebrew (private)

This repo includes a Homebrew formula at `Formula/hex.rb`. Use it as a private tap.

### 1. Authenticate with GitHub

Homebrew needs access to the private repo. Use one of:

```bash
gh auth login
```

Or configure SSH access to GitHub.

### 2. Tap the repo

HTTPS:

```bash
brew tap Janindu-Pathirana/hex https://github.com/Janindu-Pathirana/hex.git
```

SSH:

```bash
brew tap Janindu-Pathirana/hex git@github.com:Janindu-Pathirana/hex.git
```

### 3. Install

From the latest `main` branch:

```bash
brew install --HEAD hex
```

When you publish a release tag (e.g. `v0.1.0`), add `url` and `sha256` to `Formula/hex.rb`, then install with:

```bash
brew install hex
```

### Upgrade

```bash
brew update
brew upgrade hex
```
