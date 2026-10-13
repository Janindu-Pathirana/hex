---
name: go-testing
description: Testing approach for the hex Go CLI, covering table-driven unit tests, httptest fake CSE servers with JSON fixtures, golden-file tests for command output, cache tests with temp dirs, indicator math tests with tolerances, and race/coverage runs. Use when writing or fixing tests, making code testable (injecting the HTTP client/base URL), or adding fixtures under testdata/.
---

# Testing hex

There are no tests yet. Every new feature ships with tests, and you should add tests around existing code when you touch it.

```bash
go test ./...                    # fast
go test -race -cover ./...       # before commit / in CI
go test ./cmd/... -run TestMarket -update   # regenerate golden files
```

## 1. Make code testable first
- **CSE base URL and client must be injectable.** Constants like `urlCompanyProfilePost` in `cmd/company/root.go` can't point at a fake. Move them to `internal/cse.Client{BaseURL, HTTP}` (see `cse-api`), with a package-level default the commands use and tests can swap.
- **Cache dir must be overridable.** Read `HEX_CACHE_DIR` (or take a dir argument) instead of calling `os.UserCacheDir()` directly. Do the same for the config dir (`HEX_CONFIG_DIR`).
- **Time must be injectable** for TTL and market-hours logic: `var now = time.Now` or pass a `time.Time`.
- **Commands write to `cmd.OutOrStdout()`**, not `fmt.Println`, so tests can capture output.

## 2. Unit tests (pure logic)
Use table-driven tests with `t.Run(tc.name, ...)` for symbol parsing, number formatting, indicators, ratios, screener filters and portfolio math.
```go
func TestSMA(t *testing.T) {
	tests := []struct{ name string; in []float64; n int; want float64; ok bool }{
		{"basic", []float64{1,2,3,4,5,6,7,8,9,10}, 3, 9, true},
		{"too short", []float64{1,2}, 3, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := analysis.LastSMA(tc.in, tc.n)
			if ok != tc.ok || (ok && math.Abs(got-tc.want) > 1e-9) {
				t.Fatalf("got %v,%v want %v,%v", got, ok, tc.want, tc.ok)
			}
		})
	}
}
```
Test vectors for indicators are in the `technical-analysis` skill, and for ratios in `fundamental-analysis`.

## 3. Fake CSE server
```go
func fakeCSE(t *testing.T) *cse.Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/tradeSummary", serveFixture(t, "tradeSummary.json"))
	mux.HandleFunc("POST /api/companyInfoSummery", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("symbol") != "JKH.N0000" { w.Write([]byte("{}")); return } // mimic CSE: 200 + {}
		serveFixture(t, "companyInfoSummery_JKH.json")(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &cse.Client{BaseURL: srv.URL + "/api/", HTTP: srv.Client()}
}
```
- Fixtures live in `testdata/cse/<endpoint>[_<case>].json`. Capture them with the curl command in `cse-api` and **trim** to a few rows.
- Cover these paths: success, non-200, malformed JSON, empty `{}` (unknown symbol → `ErrNoData`), null fields, and context cancellation.
- Tests must **never hit the real cse.lk**. If you want a live smoke test, guard it with `//go:build live` and run it manually with `go test -tags live ./...`.

## 4. Command / golden-file tests
```go
var update = flag.Bool("update", false, "update golden files")

func runCmd(t *testing.T, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	root := cmd.NewRootCmd()          // construct fresh: avoid global flag state leaking between tests
	root.SetOut(&out); root.SetErr(&out); root.SetArgs(args)
	if err := root.Execute(); err != nil { t.Fatal(err) }
	return out.String()
}
```
- Compare against `testdata/golden/<name>.txt`. With `-update`, write the file instead of comparing.
- The global `rootCmd` + package-level flag vars in `cmd/` leak state between tests. Refactor towards a `NewRootCmd()` constructor when you add command tests.
- Force `NO_COLOR=1` (`t.Setenv`) and a fixed clock so the output is deterministic.

## 5. Cache tests
`t.Setenv("HEX_CACHE_DIR", t.TempDir())`. Cover miss → fetch → write, hit within TTL (assert the fake server saw 0 requests by counting with `atomic.Int32`), expiry (set mtime with `os.Chtimes`), and `hex clean` when the files are missing.

## 6. Conventions
- Use `t.Helper()` in helpers, `t.Cleanup` for teardown, and `t.Parallel()` only for tests with no env/global state.
- Standard library `testing` is enough. Don't add testify unless the team wants it.
- For broader strategy (fuzzing symbol parsers, `testing/quick`), also see `test-master`.
