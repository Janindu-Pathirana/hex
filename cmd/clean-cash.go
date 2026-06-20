package cmd

import (
	"fmt"

	cleancash "github.com/Janindu-Pathirana/hex/cmd/clean-cash"
	"github.com/Janindu-Pathirana/hex/internal"
	"github.com/spf13/cobra"
)

//--------------------------------

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Manage and clear local cache data",
}

var cleanAllCashCmd = &cobra.Command{
	Use:   "all",
	Short: "Clean all stored cache data",
	Run: func(cmd *cobra.Command, args []string) {
		err := cleancash.CleanAllCash()
		if err != nil {
			fmt.Println("Error cleaning cash:", err)
			return
		}
		fmt.Println("Cash cleaned successfully")
	},
}

var cleanAvailableCompaniesCashCmd = &cobra.Command{
	Use:   "available-companies",
	Short: "Clean available companies stored cache data",
	Run: func(cmd *cobra.Command, args []string) {
		err := cleancash.CleanCacheFile(internal.AvailableCompaniesCache)
		if err != nil {
			fmt.Println("Error cleaning available companies cash:", err)
			return
		}
		fmt.Println("Available companies cash cleaned successfully")
	},
}

func init() {
	cleanCmd.AddCommand(cleanAllCashCmd)
	cleanCmd.AddCommand(cleanAvailableCompaniesCashCmd)

	rootCmd.AddCommand(cleanCmd)
}
