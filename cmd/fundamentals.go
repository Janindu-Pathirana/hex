package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Janindu-Pathirana/hex/internal/extract"
	"github.com/Janindu-Pathirana/hex/internal/output"
	"github.com/Janindu-Pathirana/hex/internal/store"
	"github.com/spf13/cobra"
)

// pdfDisclaimer goes on commands that show values read from report PDFs.
const pdfDisclaimer = "Data: cse.lk report PDFs, read automatically (check low-confidence values against the PDF). For information only, not investment advice."

func newFundamentalsCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fundamentals",
		Short: "Financial statement values read from reports (or set by hand)",
	}
	cmd.AddCommand(newFundamentalsShowCmd(a), newFundamentalsSetCmd(a))
	return cmd
}

type fundamentalView struct {
	PeriodEnd  string  `json:"periodEnd"`
	PeriodType string  `json:"periodType"`
	Scope      string  `json:"scope"`
	Field      string  `json:"field"`
	Value      float64 `json:"value"`
	Source     string  `json:"source"`
	Confidence float64 `json:"confidence"`
	ReportID   int64   `json:"reportId,omitempty"`
	Page       int     `json:"page,omitempty"`
	Label      string  `json:"label,omitempty"`
}

func toFundamentalView(f store.Fundamental) fundamentalView {
	return fundamentalView{PeriodEnd: f.PeriodEnd, PeriodType: f.PeriodType, Scope: f.Scope, Field: f.Field, Value: f.Value,
		Source: f.Source, Confidence: f.Confidence, ReportID: f.ReportID, Page: f.Page, Label: f.RawLabel}
}

func newFundamentalsShowCmd(a *app) *cobra.Command {
	var all bool
	var scope, period string
	cmd := &cobra.Command{
		Use:   "show <SYMBOL>",
		Short: "Show stored statement values, newest period first",
		Long: `Show the statement values stored for a company. Amounts are in rupees;
per-share fields (eps_basic, eps_diluted, dps, nav_per_share) in rupees per
share. Balance-sheet values have period type PIT (point in time).

When several sources give a value, a manual value wins, then the highest
confidence. --all shows every source.`,
		Example:           "  hex fundamentals show jkh\n  hex fundamentals show COMB --period 2026-06-30 --scope company",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.completeSymbols,
		RunE: func(cmd *cobra.Command, args []string) error {
			sec, err := a.resolve(cmd, args[0])
			if err != nil {
				return err
			}
			db, err := store.OpenDefault(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			fs, err := db.Fundamentals(cmd.Context(), sec.Code, all)
			if err != nil {
				return err
			}
			views := []fundamentalView{}
			for _, f := range fs {
				if (scope == "" || f.Scope == scope) && (period == "" || f.PeriodEnd == period) {
					views = append(views, toFundamentalView(f))
				}
			}
			if len(fs) == 0 {
				return fmt.Errorf("no fundamentals stored for %s; run 'hex sync fundamentals --symbols %s'", sec.Code, sec.Ticker())
			}
			return output.JSON(cmd.OutOrStdout(), struct {
				Symbol     string            `json:"symbol"`
				Values     []fundamentalView `json:"values"`
				Disclaimer string            `json:"disclaimer"`
			}{sec.Code, views, pdfDisclaimer})
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Show every source, not just the winning value")
	cmd.Flags().StringVar(&scope, "scope", "", "Only group or company values")
	cmd.Flags().StringVar(&period, "period", "", "Only values for this period end (YYYY-MM-DD)")
	return cmd
}

func newFundamentalsSetCmd(a *app) *cobra.Command {
	var period, ptype, scope string
	var fields []string
	cmd := &cobra.Command{
		Use:   "set <SYMBOL> --period YYYY-MM-DD --field name=value...",
		Short: "Set statement values by hand (they override extracted ones)",
		Long: `Store values you read yourself. They take precedence over values read
from PDFs. Amounts are in rupees (not thousands); per-share fields in rupees.
Field names: ` + strings.Join(fieldNamesList(), ", ") + `.`,
		Example:           "  hex fundamentals set jkh --period 2026-06-30 --field eps_basic=0.004 --field revenue=141647460000",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.completeSymbols,
		RunE: func(cmd *cobra.Command, args []string) error {
			sec, err := a.resolve(cmd, args[0])
			if err != nil {
				return err
			}
			if _, err := time.Parse("2006-01-02", period); err != nil {
				return fmt.Errorf("--period must be YYYY-MM-DD: %w", err)
			}
			if scope != extract.ScopeGroup && scope != extract.ScopeCompany {
				return fmt.Errorf("--scope must be %s or %s", extract.ScopeGroup, extract.ScopeCompany)
			}
			var rows []store.Fundamental
			for _, kv := range fields {
				name, val, ok := strings.Cut(kv, "=")
				fd := extract.FieldByName(strings.TrimSpace(name))
				if !ok || fd == nil {
					return fmt.Errorf("--field %q: want name=value with a known field name", kv)
				}
				v, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(val), ",", ""), 64)
				if err != nil {
					return fmt.Errorf("--field %q: %w", kv, err)
				}
				pt := ptype
				if fd.Statement == extract.BalanceSheet {
					pt = extract.PointInTime
				}
				rows = append(rows, store.Fundamental{Code: sec.Code, PeriodEnd: period, PeriodType: pt, Scope: scope,
					Field: fd.Name, Source: store.SourceManual, Value: v, Confidence: 1})
			}
			if len(rows) == 0 {
				return fmt.Errorf("give at least one --field name=value")
			}
			db, err := store.OpenDefault(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			if err := db.Update(cmd.Context(), func(tx *store.Tx) error { return tx.UpsertFundamentals(cmd.Context(), rows) }); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Stored %d manual value(s) for %s %s\n", len(rows), sec.Code, period)
			return err
		},
	}
	cmd.Flags().StringVar(&period, "period", "", "Period end (YYYY-MM-DD)")
	cmd.Flags().StringVar(&ptype, "type", "3M", "Period type for income and cash-flow fields: 3M, 6M, 9M or 12M")
	cmd.Flags().StringVar(&scope, "scope", extract.ScopeGroup, "group or company")
	cmd.Flags().StringArrayVar(&fields, "field", nil, "name=value (repeatable)")
	_ = cmd.MarkFlagRequired("period")
	return cmd
}

func fieldNamesList() []string {
	out := make([]string, len(extract.Fields))
	for i, f := range extract.Fields {
		out[i] = f.Name
	}
	return out
}
