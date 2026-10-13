---
name: release
description: Release and distribution process for hex, covering semver versioning, the ldflags Version build, git tags, updating Formula/hex.rb (url + sha256) for the private Homebrew tap, verifying the brew install, and optional GoReleaser/GitHub Actions CI. Use when cutting a release, bumping the version, editing the Homebrew formula, or setting up CI.
---

# Releasing hex

## Facts
- Version variable: `cmd.Version` (`cmd/version.go`), default `"dev"`, set with `-ldflags "-X github.com/Janindu-Pathirana/hex/cmd.Version=X.Y.Z"`.
- Formula: `Formula/hex.rb`. It currently has only `head` (main branch), and the README says to add `url` + `sha256` for tagged releases.
- Repo `github.com/Janindu-Pathirana/hex` is **private**, so users need `gh auth login` or SSH to tap it.

## Checklist
1. **Clean state**: on `main`, `git status` clean, `git pull`.
2. **Quality gates**:
   ```bash
   gofmt -l . && go vet ./... && go test -race ./...
   go build -ldflags "-X github.com/Janindu-Pathirana/hex/cmd.Version=X.Y.Z" -o hex . && ./hex version && ./hex --help
   ```
3. **Pick a version** (semver, `v` prefix on tags): breaking CLI change (renamed command or flag, changed `--json` shape) → minor while 0.x, major after 1.0. New command → minor. Fix → patch.
4. **Changelog**: add a section to `CHANGELOG.md` (create it if missing) from `git log --oneline <last-tag>..HEAD`.
5. **Tag**: `git tag -a vX.Y.Z -m "hex vX.Y.Z" && git push origin vX.Y.Z`. Pushing is outward-facing, so confirm with the user first.
6. **Formula**: compute the source tarball checksum. The repo is private, so download with auth:
   ```bash
   gh api repos/Janindu-Pathirana/hex/tarball/vX.Y.Z > /tmp/hex-vX.Y.Z.tar.gz
   shasum -a 256 /tmp/hex-vX.Y.Z.tar.gz
   ```
   Then update `Formula/hex.rb`:
   ```ruby
   url "https://github.com/Janindu-Pathirana/hex/archive/refs/tags/vX.Y.Z.tar.gz"
   sha256 "<sha>"
   version "X.Y.Z"   # only if brew can't infer it from the url
   head "https://github.com/Janindu-Pathirana/hex.git", branch: "main"
   ```
   Note: for private repos the plain archive URL needs auth. If `brew install` fails to download, a custom `GitHubPrivateRepositoryDownloadStrategy` (with `HOMEBREW_GITHUB_API_TOKEN`) or making the repo public is required. Record which approach was chosen in the README.
   Commit the formula change: `release: vX.Y.Z`.
7. **Verify**:
   ```bash
   brew update && brew upgrade hex   # or: brew reinstall hex
   hex version                        # must print X.Y.Z, not "dev"
   brew test hex
   brew audit --strict hex            # fix warnings where reasonable
   ```
   Also improve the formula `test do` block to check the version: `assert_match version.to_s, shell_output("#{bin}/hex version")`.

## Build hygiene
- Add `-trimpath` and `-s -w` to release builds: `go build -trimpath -ldflags "-s -w -X ...Version=X.Y.Z"`.
- Embed tzdata (`import _ "time/tzdata"`) so Asia/Colombo works everywhere (see `terminal-output`).
- The `hex` binary in the repo root is git-ignored, so never commit it.

## Optional automation
- **CI** (`.github/workflows/ci.yml`): on push/PR run `go vet`, `go test -race ./...`, `gofmt -l` (fail if output), and optionally `golangci-lint` (`golangci/golangci-lint-action`). Use `actions/setup-go` with `go-version-file: go.mod`.
- **GoReleaser** (`.goreleaser.yaml`): builds darwin/linux amd64+arm64 with the ldflags, creates the GitHub release, and can update the formula in this repo's `Formula/` directory automatically (`brews:` section). Use the `devops-engineer` skill for pipeline details.
- Workflow on tag push `v*`: run GoReleaser with `GITHUB_TOKEN`.
