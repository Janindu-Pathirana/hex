package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "hex",
	Short: "Analyze companies on the Sri Lanka stock market",
	Long:  "hex is a CLI tool for analyzing companies listed on the Sri Lanka stock market.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
