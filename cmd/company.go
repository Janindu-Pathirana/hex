package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Janindu-Pathirana/hex/cmd/company"
	"github.com/Janindu-Pathirana/hex/internal/cse"
	"github.com/Janindu-Pathirana/hex/internal/output"
	"github.com/spf13/cobra"
)

func newCompanyCmd(a *app) *cobra.Command {
	companyCmd := &cobra.Command{
		Use:   "company",
		Short: "Fetch and display company data",
	}
	companyCmd.AddCommand(
		newCompanyListCmd(a),
		newCompanyProfileCmd(a),
		newCompanyFinancialCmd(a),
	)
	return companyCmd
}

func newCompanyListCmd(a *app) *cobra.Command {
	var namesOnly, symbolsOnly bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all available companies",
		Example: `  hex company list
  hex company list --symbols`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			secs, err := company.Securities(cmd.Context(), a.cse)
			if err != nil {
				return err
			}

			var lines []string
			switch {
			case namesOnly:
				seen := make(map[string]bool)
				for _, s := range secs {
					if !seen[s.Name] {
						seen[s.Name] = true
						lines = append(lines, strings.ReplaceAll(s.Name, " ", "-"))
					}
				}
			case symbolsOnly:
				lines = cse.Tickers(secs)
			default:
				for _, s := range secs {
					lines = append(lines, s.Name+","+s.Code)
				}
			}

			w := cmd.OutOrStdout()
			for _, l := range lines {
				if _, err := fmt.Fprintln(w, l); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().SetNormalizeFunc(legacyFlags(map[string]string{
		"onlyNames":  "names",
		"onlySymbol": "symbols",
	}))
	cmd.Flags().BoolVarP(&namesOnly, "names", "n", false, "Print only company names")
	cmd.Flags().BoolVarP(&symbolsOnly, "symbols", "s", false, "Print only ticker symbols")
	cmd.MarkFlagsMutuallyExclusive("names", "symbols")
	return cmd
}

func newCompanyProfileCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:               "show-profile <SYMBOL>",
		Aliases:           []string{"profile"},
		Short:             "Show company profile",
		Example:           "  hex company show-profile JKH\n  hex company profile cic.x0000",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.completeSymbols,
		RunE: func(cmd *cobra.Command, args []string) error {
			sec, err := a.resolve(cmd, args[0])
			if err != nil {
				return err
			}
			profile, err := a.cse.CompanyProfile(cmd.Context(), sec.Code)
			if err != nil {
				return fmt.Errorf("fetch company profile for %s: %w", sec.Code, err)
			}
			return output.JSON(cmd.OutOrStdout(), profile)
		},
	}
}

// financialSections are the mutually exclusive section flags of show-financial.
var financialSections = []string{"annual", "quarterly", "other", "banner-ad", "web-link", "req-financial"}

func newCompanyFinancialCmd(a *app) *cobra.Command {
	show := make(map[string]*bool, len(financialSections))
	var companyBannerAd bool

	cmd := &cobra.Command{
		Use:     "show-financial <SYMBOL>",
		Aliases: []string{"financials", "reports"},
		Short:   "Show company financial reports (PDF links)",
		Long: `Show the financial reports a company has filed with the CSE.

Without a section flag every section is shown. Report entries include the
report date, upload/authorisation times (Asia/Colombo) and the PDF URL.`,
		Example:           "  hex company show-financial JKH\n  hex company show-financial JKH --quarterly",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.completeSymbols,
		RunE: func(cmd *cobra.Command, args []string) error {
			if companyBannerAd {
				*show["banner-ad"] = true
			}

			sec, err := a.resolve(cmd, args[0])
			if err != nil {
				return err
			}
			f, err := a.cse.Financials(cmd.Context(), sec.Code)
			if err != nil {
				return fmt.Errorf("fetch financial data for %s: %w", sec.Code, err)
			}

			var data any
			switch {
			case *show["annual"]:
				data = company.ReportViews(f.Annual)
			case *show["quarterly"]:
				data = company.ReportViews(f.Quarterly)
			case *show["other"]:
				data = company.ReportViews(f.Other)
			case *show["banner-ad"]:
				data = f.BannerAd
			case *show["web-link"]:
				data = f.WebLinks
			case *show["req-financial"]:
				data = f.Extra
			default:
				data = struct {
					Symbol       string               `json:"symbol"`
					Annual       []company.ReportView `json:"annual"`
					Quarterly    []company.ReportView `json:"quarterly"`
					Other        []company.ReportView `json:"other"`
					WebLinks     []*string            `json:"webLinks"`
					ReqFinancial []cse.ReqFinancial   `json:"reqFinancial"`
					BannerAd     cse.Asset            `json:"bannerAd"`
				}{
					Symbol:       sec.Code,
					Annual:       company.ReportViews(f.Annual),
					Quarterly:    company.ReportViews(f.Quarterly),
					Other:        company.ReportViews(f.Other),
					WebLinks:     f.WebLinks,
					ReqFinancial: f.Extra,
					BannerAd:     f.BannerAd,
				}
			}
			return output.JSON(cmd.OutOrStdout(), data)
		},
	}

	cmd.Flags().SetNormalizeFunc(legacyFlags(map[string]string{
		"showAnnualData":    "annual",
		"showQuarterlyData": "quarterly",
		"showOtherData":     "other",
		"showBannerAd":      "banner-ad",
		"showWebLink":       "web-link",
		"showReqFinancial":  "req-financial",
	}))
	usage := map[string][2]string{
		"annual":        {"a", "Show only annual reports"},
		"quarterly":     {"q", "Show only quarterly reports"},
		"other":         {"o", "Show only other filings"},
		"banner-ad":     {"b", "Show only the company banner ad"},
		"web-link":      {"w", "Show only web links"},
		"req-financial": {"r", "Show only extra financial data elements"},
	}
	for _, name := range financialSections {
		show[name] = new(bool)
		cmd.Flags().BoolVarP(show[name], name, usage[name][0], false, usage[name][1])
	}
	cmd.Flags().BoolVarP(&companyBannerAd, "showCompanyBannerAd", "c", false, "Show company banner ad")
	_ = cmd.Flags().MarkDeprecated("showCompanyBannerAd", "use --banner-ad (-b); both print the same field")
	cmd.MarkFlagsMutuallyExclusive(financialSections...)
	return cmd
}

// resolve maps a user-typed symbol to a listed security with a helpful error.
func (a *app) resolve(cmd *cobra.Command, input string) (cse.Security, error) {
	sec, err := company.Resolve(cmd.Context(), a.cse, input)
	if errors.Is(err, cse.ErrUnknownSymbol) {
		return sec, fmt.Errorf("%w. Use 'hex company list --symbols' to see valid symbols", err)
	}
	return sec, err
}
