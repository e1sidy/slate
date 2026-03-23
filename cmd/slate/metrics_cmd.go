package main

import (
	"fmt"
	"time"

	"github.com/e1sidy/slate"
	"github.com/spf13/cobra"
)

func metricsCmd() *cobra.Command {
	var fromStr, toStr, actor string

	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Show task metrics (cycle time, throughput)",
		RunE: func(cmd *cobra.Command, args []string) error {
			p := slate.MetricsParams{}
			if actor != "" {
				p.Actor = actor
			}
			if fromStr != "" {
				t, err := time.Parse("2006-01-02", fromStr)
				if err != nil {
					return fmt.Errorf("invalid --from date: %w", err)
				}
				p.From = &t
			}
			if toStr != "" {
				t, err := time.Parse("2006-01-02", toStr)
				if err != nil {
					return fmt.Errorf("invalid --to date: %w", err)
				}
				end := t.Add(24*time.Hour - time.Second) // end of day
				p.To = &end
			}

			report, err := store.Metrics(cmd.Context(), p)
			if err != nil {
				return err
			}

			if jsonOutput {
				return printJSON(report)
			}

			fmt.Printf("Tasks created:    %d\n", report.TasksCreated)
			fmt.Printf("Tasks closed:     %d\n", report.TasksClosed)
			fmt.Printf("Tasks cancelled:  %d\n", report.TasksCancelled)
			fmt.Printf("Currently open:   %d\n", report.CurrentOpen)
			fmt.Printf("Currently blocked: %d\n", report.CurrentBlocked)
			if report.AvgCycleTime > 0 {
				fmt.Printf("Avg cycle time:   %s\n", report.AvgCycleTime.Round(time.Minute))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&fromStr, "from", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&toStr, "to", "", "End date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&actor, "actor", "", "Filter by actor")

	return cmd
}
