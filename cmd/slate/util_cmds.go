package main

import (
	"fmt"

	"github.com/e1sidy/slate"
	"github.com/spf13/cobra"
)

func statsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show task statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Count by status.
			type stat struct {
				label string
				count int
			}
			statuses := []struct {
				s slate.Status
				l string
			}{
				{slate.StatusOpen, "Open"},
				{slate.StatusInProgress, "In Progress"},
				{slate.StatusBlocked, "Blocked"},
				{slate.StatusDeferred, "Deferred"},
				{slate.StatusClosed, "Closed"},
				{slate.StatusCancelled, "Cancelled"},
			}

			var total int
			results := make([]stat, 0, len(statuses))

			for _, st := range statuses {
				tasks, err := store.List(ctx, slate.ListParams{Status: &st.s})
				if err != nil {
					continue
				}
				results = append(results, stat{st.l, len(tasks)})
				total += len(tasks)
			}

			if jsonOutput {
				data := map[string]int{"total": total}
				for _, r := range results {
					data[r.label] = r.count
				}
				return printJSON(data)
			}

			fmt.Printf("Total tasks: %d\n\n", total)
			for _, r := range results {
				fmt.Printf("  %-14s %d\n", r.label+":", r.count)
			}
			return nil
		},
	}
}

func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run health checks",
		RunE: func(cmd *cobra.Command, args []string) error {
			report, err := store.Doctor(cmd.Context())
			if err != nil {
				return err
			}

			if jsonOutput {
				return printJSON(report)
			}

			for _, d := range report.Diagnostics {
				icon := "✓"
				color := green
				switch d.Level {
				case slate.DiagWarn:
					icon = "⚠"
					color = yellow
				case slate.DiagFail:
					icon = "✗"
					color = red
				}
				fmt.Printf("%s %s: %s\n", colorize(color, icon), d.Name, d.Message)
			}

			if report.HasIssues() {
				fmt.Println("\nSome checks reported issues.")
			} else {
				fmt.Println("\nAll checks passed.")
			}
			return nil
		},
	}
}
