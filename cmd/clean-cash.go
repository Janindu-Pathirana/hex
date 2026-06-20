package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Janindu-Pathirana/hex/internal"
	"github.com/spf13/cobra"
)

var cleanCacheFile = func(cashInfo internal.CacheConfig) error {

	path, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("resolve cache directory: %w", err)
	}
	path = filepath.Join(path, internal.CacheDir, cashInfo.CacheFile)

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove cache file: %w", err)
	}

	return nil
}

var cleanAllCash = func() error {
	err := cleanCacheFile(internal.AvailableCompaniesCache)
	if err != nil {
		return fmt.Errorf("clean available companies cache: %w", err)
	}

	return nil
}

//--------------------------------

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Manage and clear local cache data",
}

var cleanAllCashCmd = &cobra.Command{
	Use:   "all",
	Short: "Clean all stored cache data",
	Run: func(cmd *cobra.Command, args []string) {
		err := cleanAllCash()
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
		err := cleanCacheFile(internal.AvailableCompaniesCache)
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
