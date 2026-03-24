package main

import (
	"fmt"
	"os"
	"time"

	"github.com/e1sidy/slate"
	"github.com/spf13/cobra"
)

func archiveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Archive and restore old tasks",
	}
	cmd.AddCommand(archiveRunCmd(), unarchiveCmd(), archivedCmd())
	return cmd
}

func archiveRunCmd() *cobra.Command {
	var before string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Archive closed tasks older than a date",
		RunE: func(cmd *cobra.Command, args []string) error {
			cutoff := time.Now().AddDate(0, 0, -90) // default: 90 days
			if before != "" {
				t, err := time.Parse("2006-01-02", before)
				if err != nil {
					return fmt.Errorf("invalid date %q: %w", before, err)
				}
				cutoff = t
			}

			archivePath := slate.DefaultArchivePath()
			result, err := store.Archive(cmd.Context(), cutoff, archivePath)
			if err != nil {
				return err
			}

			if jsonOutput {
				return printJSON(result)
			}

			fmt.Fprintf(os.Stderr, "Archived %d tasks to %s\n", result.Archived, archivePath)
			return nil
		},
	}

	cmd.Flags().StringVar(&before, "before", "", "Archive tasks closed before this date (YYYY-MM-DD, default: 90 days ago)")
	return cmd
}

func unarchiveCmd() *cobra.Command {
	var taskID string

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore tasks from archive",
		RunE: func(cmd *cobra.Command, args []string) error {
			archivePath := slate.DefaultArchivePath()
			var taskIDs []string
			if taskID != "" {
				taskIDs = []string{taskID}
			}

			restored, err := store.Unarchive(cmd.Context(), archivePath, taskIDs)
			if err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Restored %d tasks from archive\n", restored)
			return nil
		},
	}

	cmd.Flags().StringVar(&taskID, "task", "", "Restore a specific task by ID (default: all)")
	return cmd
}

func archivedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List archived tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			archivePath := slate.DefaultArchivePath()
			tasks, err := slate.ListArchived(cmd.Context(), archivePath)
			if err != nil {
				return err
			}

			if tasks == nil || len(tasks) == 0 {
				fmt.Fprintln(os.Stderr, "No archived tasks")
				return nil
			}

			if jsonOutput {
				return printJSON(tasks)
			}

			for _, t := range tasks {
				fmt.Printf("%s  %s  %s  %s\n", t.ID, t.Status, t.CloseReason, t.Title)
			}
			return nil
		},
	}
}
