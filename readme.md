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
