// Package output formats hex command output.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Colombo is the Asia/Colombo time zone (UTC+05:30, no DST).
var Colombo = loadColombo()

func loadColombo() *time.Location {
	loc, err := time.LoadLocation("Asia/Colombo")
	if err != nil {
		return time.FixedZone("SLST", 5*3600+1800)
	}
	return loc
}

// JSON writes v as indented JSON followed by a newline.
func JSON(w io.Writer, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	if _, err := fmt.Fprintln(w, string(b)); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

// Date formats a Unix-millisecond timestamp as a Colombo date (2006-01-02).
func Date(ms int64) string {
	return time.UnixMilli(ms).In(Colombo).Format("2006-01-02")
}

// DateTime formats a Unix-millisecond timestamp as Colombo date and time.
func DateTime(ms int64) string {
	return time.UnixMilli(ms).In(Colombo).Format("2006-01-02 15:04")
}
