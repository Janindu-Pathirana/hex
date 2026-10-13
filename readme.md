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

## Usage

See [FEATURES.md](FEATURES.md) for the full feature reference.

```bash
hex company list                      # NAME,CODE for every listed security (N, X and U series)
hex company list --symbols            # unique tickers
hex company show-profile jkh          # profile JSON; symbols are case-insensitive
hex company profile CIC.X0000         # full codes select a specific series
hex company show-financial JKH        # all report sections with PDF URLs
hex company show-financial JKH -q     # only quarterly reports (-a annual, -o other, -w web links, -r extra, -b banner)
hex sync                              # market, symbol list and today's quotes into the local database
hex sync --symbols jkh,comb           # plus company info, history, profile and report catalogue
hex sync fundamentals --symbols jkh   # download the newest interim reports and read their statements
hex sync fundamentals --symbols jkh --llm   # also ask Claude about low-confidence or scanned reports (ANTHROPIC_API_KEY)
hex sync all --all-symbols            # everything for every listed security (slow; rate-limited)
hex sync status                       # what is stored, how fresh, what failed
hex reports list jkh                  # catalogued reports and their processing state
hex reports extract jkh --show-pages  # read one report now and show the values
hex fundamentals show jkh             # statement values with source, page and confidence
hex fundamentals set jkh --period 2026-06-30 --field eps_basic=0.004   # manual override
hex clean all                         # remove cached data (the database is kept)
hex clean db                          # delete the local database
hex completion zsh > "${fpath[1]}/_hex"   # shell completion (also bash, fish, powershell)
```

Errors go to stderr and exit with status 1. The symbol list and report PDFs are cached under the user cache directory (`HEX_CACHE_DIR` overrides it). Synced data lives in `hex.db` under the user config directory (`~/Library/Application Support/hex` on macOS; `HEX_DATA_DIR` overrides it). `hex sync` skips work that is still fresh and resumes after an interruption; schedule it with cron or launchd for daily updates. Values read from PDFs are informational, not investment advice; check low-confidence ones against the PDF.

## Test

```bash
go vet ./... && go test -race -cover ./...
go test ./cmd -update                 # regenerate golden files after an intended output change
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
