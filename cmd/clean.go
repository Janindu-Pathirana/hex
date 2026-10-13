package cmd

import (
	"fmt"

	"github.com/Janindu-Pathirana/hex/internal"
	"github.com/Janindu-Pathirana/hex/internal/cache"
	"github.com/Janindu-Pathirana/hex/internal/store"
	"github.com/spf13/cobra"
)

func newCleanCmd() *cobra.Command {
	cleanCmd := &cobra.Command{
		Use:   "clean",
		Short: "Manage and clear local cache data",
	}

	cleanCmd.AddCommand(
		&cobra.Command{
			Use:   "all",
			Short: "Clean all stored cache data",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := cache.RemoveAll(); err != nil {
					return fmt.Errorf("clean cache: %w", err)
				}
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "Cache cleaned successfully")
				return err
			},
		},
		&cobra.Command{
			Use:     "securities",
			Aliases: []string{"available-companies"},
			Short:   "Clean the cached company symbol list",
			Args:    cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := cache.Remove(internal.SecuritiesCache); err != nil {
					return fmt.Errorf("clean securities cache: %w", err)
				}
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "Securities cache cleaned successfully")
				return err
			},
		},
		&cobra.Command{
			Use:   "db",
			Short: "Delete the local database (everything 'hex sync' stored, including manual values)",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := store.RemoveFiles(); err != nil {
					return fmt.Errorf("clean database: %w", err)
				}
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "Database deleted")
				return err
			},
		},
		&cobra.Command{
			Use:   "raw",
			Short: "Delete the raw cse.lk responses kept in the database",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				db, err := store.OpenDefault(cmd.Context())
				if err != nil {
					return err
				}
				defer db.Close()
				n, err := db.ClearRaw(cmd.Context())
				if err != nil {
					return err
				}
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Deleted %d raw responses\n", n)
				return err
			},
		},
	)
	return cleanCmd
}
