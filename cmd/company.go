package cmd

import (
	"fmt"

	"strings"

	"github.com/Janindu-Pathirana/hex/cmd/company"
	customStructs "github.com/Janindu-Pathirana/hex/internal/custom-structs"
	"github.com/Janindu-Pathirana/hex/internal/util"
	"github.com/spf13/cobra"
)

var onlyNames bool
var onlySymbol bool

var showAnnualData bool
var showQuarterlyData bool
var showOtherData bool
var showBannerAd bool
var showWebLink bool
var showReqFinancial bool
var showCompanyBannerAd bool

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

var showCompanyFinancialDataCmd = &cobra.Command{
	Use: "show-financial",
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
	Short:     "Show company financial data",
	Run: func(cmd *cobra.Command, args []string) {
		companySymbol := args[0]
		companyFinancialData, err := company.FetchCompanyFinancialDataBySymbol(companySymbol)
		if err != nil {
			fmt.Println("Error fetching company financial data:", err)
			return
		}

		var dataToShow interface{}

		if showAnnualData {
			var makeAnnualData = make([]customStructs.InfoAnnualData, len(companyFinancialData.InfoAnnualData))
			for i, data := range companyFinancialData.InfoAnnualData {
				makeAnnualData[i] = customStructs.InfoAnnualData{
					ManualDate:     data.ManualDate,
					UploadedDate:   data.UploadedDate,
					FileText:       data.FileText,
					AuthorizedDate: data.AuthorizedDate,
				}
			}
			dataToShow = makeAnnualData
		}
		if showQuarterlyData {
			var makeQuarterlyData = make([]customStructs.InfoQuarterlyData, len(companyFinancialData.InfoQuarterlyData))
			for i, data := range companyFinancialData.InfoQuarterlyData {
				makeQuarterlyData[i] = customStructs.InfoQuarterlyData{
					ManualDate:     data.ManualDate,
					UploadedDate:   data.UploadedDate,
					FileText:       data.FileText,
					AuthorizedDate: data.AuthorizedDate,
				}
			}
			dataToShow = makeQuarterlyData
		}
		if showOtherData {
			var makeOtherData = make([]customStructs.InfoOtherData, len(companyFinancialData.InfoOtherData))
			for i, data := range companyFinancialData.InfoOtherData {
				makeOtherData[i] = customStructs.InfoOtherData{
					ManualDate:     data.ManualDate,
					UploadedDate:   data.UploadedDate,
					FileText:       data.FileText,
					AuthorizedDate: data.AuthorizedDate,
				}
			}
			dataToShow = makeOtherData
		}
		if showBannerAd {
			dataToShow = companyFinancialData.InfoCompanyBannerAd
		}
		if showWebLink {
			dataToShow = companyFinancialData.InfoWebLink
		}
		if showReqFinancial {

			dataToShow = companyFinancialData.ReqFinancial
		}
		if showCompanyBannerAd {
			dataToShow = companyFinancialData.InfoCompanyBannerAd
		}
		if dataToShow == nil {
			util.PrettyPrintJSON(companyFinancialData)

		} else {
			util.PrettyPrintJSON(dataToShow)
		}

	},
}

func init() {

	listCmd.Flags().BoolVarP(&onlyNames, "onlyNames", "n", false, "Print only company names ")
	listCmd.Flags().BoolVarP(&onlySymbol, "onlySymbol", "s", false, "Print only company symbol ")

	showCompanyFinancialDataCmd.Flags().BoolVarP(&showAnnualData, "showAnnualData", "a", false, "Show annual data")
	showCompanyFinancialDataCmd.Flags().BoolVarP(&showQuarterlyData, "showQuarterlyData", "q", false, "Show quarterly data")
	showCompanyFinancialDataCmd.Flags().BoolVarP(&showOtherData, "showOtherData", "o", false, "Show other data")
	showCompanyFinancialDataCmd.Flags().BoolVarP(&showBannerAd, "showBannerAd", "b", false, "Show banner ad")
	showCompanyFinancialDataCmd.Flags().BoolVarP(&showWebLink, "showWebLink", "w", false, "Show web link")
	showCompanyFinancialDataCmd.Flags().BoolVarP(&showReqFinancial, "showReqFinancial", "r", false, "Show req financial")
	showCompanyFinancialDataCmd.Flags().BoolVarP(&showCompanyBannerAd, "showCompanyBannerAd", "c", false, "Show company banner ad")

	rootCmd.AddCommand(companyCmd)
	companyCmd.AddCommand(listCmd)
	companyCmd.AddCommand(showCompanyProfileCmd)
	companyCmd.AddCommand(showCompanyFinancialDataCmd)
}
