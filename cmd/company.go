package cmd

import (
	"fmt"

	"strings"

	"github.com/Janindu-Pathirana/hex/cmd/company"
	"github.com/Janindu-Pathirana/hex/internal/util"
	"github.com/spf13/cobra"
)

var onlyNames bool
var onlySymbol bool

var getValidArgs = func() []string {
	symbols, err := company.FetchAvailableCompanySymbols()

	if err != nil {
		return nil
	}

	return symbols
}

var companyCmd = &cobra.Command{
	Use:   "company",
	Short: "Fetch and display company data",
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available companies",
	Run: func(cmd *cobra.Command, args []string) {

		if onlyNames {
			companies, err := company.FetchAvailableCompanyNames()
			if err != nil {
				fmt.Println("Error fetching available companies:", err)
				return
			}
			for _, name := range companies {
				fmt.Println(strings.ReplaceAll(name, " ", "-"))
			}
			return
		} else if onlySymbol {

			symbols, err := company.FetchAvailableCompanySymbols()
			if err != nil {
				fmt.Println("Error fetching available companies:", err)
				return
			}
			for _, symbol := range symbols {
				fmt.Println(symbol)
			}
			return

		} else {

			companies, err := company.FetchAllAvailableCompanies()
			if err != nil {
				fmt.Println("Error fetching available companies:", err)
				return
			}

			for _, name := range companies {
				fmt.Println(name)
			}
		}
	},
}

var showCompanyProfileCmd = &cobra.Command{
	Use: "show-profile",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("accepts 1 arg(s), received %d", len(args))
		}

		validList := getValidArgs()
		input := args[0]

		for _, valid := range validList {
			if input == valid {
				return nil
			}
		}

		return fmt.Errorf("invalid company symbol %q. Use 'hex company list --onlySymbol' to see valid names", input)
	},
	ValidArgs: getValidArgs(),
	Short:     "Show company profile",
	Run: func(cmd *cobra.Command, args []string) {
		companySymbol := args[0]
		companyProfile, err := company.FetchCompanyProfileBySymbol(companySymbol)
		if err != nil {
			fmt.Println("Error fetching company profile:", err)
			return
		}

		util.PrettyPrintJSON(companyProfile)
	},
}

func init() {

	listCmd.Flags().BoolVarP(&onlyNames, "onlyNames", "n", false, "Print only company names ")
	listCmd.Flags().BoolVarP(&onlySymbol, "onlySymbol", "s", false, "Print only company symbol ")

	rootCmd.AddCommand(companyCmd)
	companyCmd.AddCommand(listCmd)
	companyCmd.AddCommand(showCompanyProfileCmd)
}
