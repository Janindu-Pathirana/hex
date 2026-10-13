---
name: terminal-output
description: Output formatting standards for the hex CLI: aligned tables, --json machine output, LKR/number/percent formatting, gain/loss colouring respecting NO_COLOR and non-TTY, sparklines and ASCII charts, and Asia/Colombo date display. Use when printing anything from a hex command, replacing JSON dumps with readable output, or adding charts to the terminal.
---

# Terminal output for hex

Today commands print JSON with `output.JSON` (`internal/output`). The target is a human-readable table by default and `--json` for scripts.

## Shape
Create `internal/output` with:
```go
// Render prints v as indented JSON when asJSON, otherwise the table.
func Render(w io.Writer, asJSON bool, v any, t Table) error
type Table struct { Headers []string; Rows [][]string; Align []Align }
```
- Use `text/tabwriter` (stdlib) for tables: `tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)`. Right-align numeric columns by padding yourself, or with the `tabwriter.AlignRight` flag on a numeric-only writer.
- Only take a dependency (e.g. `olekukonko/tablewriter`, `charmbracelet/lipgloss`) if borders or wrapping are really needed.
- JSON output must be stable: emit the typed struct, not the raw API payload, so a CSE schema change doesn't break user scripts.
- Write errors and progress to **stderr**, data to **stdout**.

## Numbers
- Prices: 2 decimals (`1,051.75`). CSE tick sizes are small and some penny stocks trade below 1, so never round to whole numbers.
- Thousands separators: write a small helper, since the stdlib has none (`golang.org/x/text/message` is the alternative).
- Large values: `12.3 Mn`, `4.56 Bn`, `1.2 Tn` (Sri Lankan market reports use Mn/Bn). Turnover is shown in LKR: `Rs. 1.19 Bn` or `LKR 1.19 Bn`. Pick one and use it everywhere.
- Percentages: signed, 2 decimals, `+2.81%` / `-0.11%`.
- Missing or null values print `-`, never `0`.

## Colour
- Green for positive change, red for negative, default for zero.
- Disable when `NO_COLOR` is set, when stdout isn't a TTY (`golang.org/x/term.IsTerminal(int(os.Stdout.Fd()))`), or when `--no-color` is passed. Never colour `--json` output.
- Use raw ANSI codes (`\x1b[32m`), which needs no dependency.

## Dates
```go
var colombo = mustLoad("Asia/Colombo") // time.LoadLocation; fall back to time.FixedZone("SLST", 5*3600+1800)
time.UnixMilli(ms).In(colombo).Format("2006-01-02 15:04")
```
Daily bars: format the date only (`2006-01-02`). Embed tzdata with `import _ "time/tzdata"` in `main.go` so Homebrew builds work on any machine.

## Charts in the terminal
- Sparkline from closes: map values onto `▁▂▃▄▅▆▇█`. This is good for `hex price JKH --history` or watchlist rows.
- For bigger charts, consider `guptarohit/asciigraph` (small and dependency-free). For colour and design choices, see the `dataviz` skill.

## Disclaimer
Analysis commands (`analyze`, `screen`, `portfolio`) end their human output with a dim one-liner:
`Data: cse.lk (may be delayed). For information only — not investment advice.`
Leave it out of `--json`.
