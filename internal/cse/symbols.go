package cse

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

const symbolListPath = "chat/company_symbol_list.csv"

// ErrUnknownSymbol is returned when a symbol does not match any listed security.
var ErrUnknownSymbol = errors.New("unknown symbol")

// Security is one listed security, e.g. {"JOHN KEELLS HOLDINGS PLC", "JKH.N0000"}.
type Security struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

// Ticker is the part of the code before the dot ("JKH").
func (s Security) Ticker() string {
	ticker, _, _ := strings.Cut(s.Code, ".")
	return ticker
}

// Series is the part of the code after the dot ("N0000"), or "" if there is none.
func (s Security) Series() string {
	_, series, _ := strings.Cut(s.Code, ".")
	return series
}

// Securities fetches every listed security (all series: N, X, U…).
func (c *Client) Securities(ctx context.Context) ([]Security, error) {
	body, err := c.get(ctx, symbolListPath)
	if err != nil {
		return nil, err
	}
	secs, err := ParseSymbolList(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse company symbol list: %w", err)
	}
	return secs, nil
}

// ParseSymbolList parses the "Company Name,Symbol" CSV published by cse.lk.
func ParseSymbolList(r io.Reader) ([]Security, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true

	records, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, errors.New("missing data rows")
	}

	secs := make([]Security, 0, len(records)-1)
	for _, rec := range records[1:] {
		if len(rec) < 2 {
			continue
		}
		name := strings.TrimSpace(rec[0])
		code := strings.ToUpper(strings.TrimSpace(rec[len(rec)-1]))
		if code == "" {
			continue
		}
		secs = append(secs, Security{Name: name, Code: code})
	}
	if len(secs) == 0 {
		return nil, errors.New("no companies found")
	}
	return secs, nil
}

// Resolve maps user input to a listed security. It is case-insensitive and
// accepts a ticker ("jkh", which prefers the voting N0000 series) or a full
// code ("CIC.X0000").
func Resolve(input string, secs []Security) (Security, error) {
	want := strings.ToUpper(strings.TrimSpace(input))
	if want == "" {
		return Security{}, fmt.Errorf("%w: empty symbol", ErrUnknownSymbol)
	}

	if strings.Contains(want, ".") {
		for _, s := range secs {
			if s.Code == want {
				return s, nil
			}
		}
		return Security{}, fmt.Errorf("%w %q", ErrUnknownSymbol, input)
	}

	var found []Security
	for _, s := range secs {
		if s.Ticker() == want {
			found = append(found, s)
		}
	}
	if len(found) == 0 {
		return Security{}, fmt.Errorf("%w %q", ErrUnknownSymbol, input)
	}
	for _, s := range found {
		if s.Series() == "N0000" {
			return s, nil
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Code < found[j].Code })
	return found[0], nil
}

// Tickers returns the unique tickers in list order.
func Tickers(secs []Security) []string {
	seen := make(map[string]bool, len(secs))
	out := make([]string, 0, len(secs))
	for _, s := range secs {
		if t := s.Ticker(); !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}
