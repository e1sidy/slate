package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func readyCmd() *cobra.Command {
	var parent string
	cmd := &cobra.Command{
		Use:   "ready",
		Short: "List tasks with no unresolved blockers",
		RunE: func(cmd *cobra.Command, args []string) error {
			tasks, err := store.Ready(cmd.Context(), parent)
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(tasks)
			}
			for _, t := range tasks {
				line := fmt.Sprintf("%s %s %s %s",
					colorStatus(t.Status), colorPriority(t.Priority), t.Title, colorID(t.ID))
				if t.Assignee != "" {
					line += " " + colorAssignee(t.Assignee)
				}
				fmt.Println(line)
			}
			if len(tasks) == 0 {
				fmt.Println("No ready tasks.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&parent, "parent", "", "Filter by parent ID")
	return cmd
}

func blockedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "blocked",
		Short: "List blocked tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			tasks, err := store.Blocked(cmd.Context())
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(tasks)
			}
			for _, t := range tasks {
				fmt.Printf("%s %s %s %s\n",
					colorStatus(t.Status), colorPriority(t.Priority), t.Title, colorID(t.ID))
			}
			if len(tasks) == 0 {
				fmt.Println("No blocked tasks.")
			}
			return nil
		},
	}
}

func childrenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "children <id>",
		Short: "List direct children of a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tasks, err := store.Children(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(tasks)
			}
			for _, t := range tasks {
				fmt.Printf("%s %s %s %s\n",
					colorStatus(t.Status), colorPriority(t.Priority), t.Title, colorID(t.ID))
			}
			if len(tasks) == 0 {
				fmt.Println("No children.")
			}
			return nil
		},
	}
}

func nextCmd() *cobra.Command {
	var criticalPath bool

	cmd := &cobra.Command{
		Use:   "next",
		Short: "Suggest the highest-impact ready task to work on",
		RunE: func(cmd *cobra.Command, args []string) error {
			if criticalPath {
				result, err := store.CriticalPath(cmd.Context())
				if err != nil {
					return err
				}
				if jsonOutput {
					return printJSON(result)
				}
				if len(result.Path) > 0 {
					fmt.Printf("Critical path (%dh estimated):\n", result.TotalEstimate)
					for i, t := range result.Path {
						arrow := ""
						if i > 0 {
							arrow = "  → "
						}
						fmt.Printf("%s%s %s (%s)\n", arrow, colorID(t.ID), t.Title, colorStatus(t.Status))
					}
				}
				if len(result.Bottlenecks) > 0 {
					fmt.Printf("\nBottlenecks:\n")
					for _, t := range result.Bottlenecks {
						fmt.Printf("  %s %s (unblocks downstream)\n", colorID(t.ID), t.Title)
					}
				}
				if len(result.Parallel) > 0 {
					fmt.Printf("\nParallelizable:\n")
					for _, t := range result.Parallel {
						fmt.Printf("  %s %s\n", colorID(t.ID), t.Title)
					}
				}
				return nil
			}

			task, err := store.Next(cmd.Context())
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(task)
			}
			fmt.Printf("Recommended: %s %s %s %s\n",
				colorStatus(task.Status), colorPriority(task.Priority), task.Title, colorID(task.ID))
			return nil
		},
	}
	cmd.Flags().BoolVar(&criticalPath, "critical-path", false, "Show full critical path analysis")
	return cmd
}

func eventsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "events <id>",
		Short: "Show event audit log for a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			events, err := store.Events(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(events)
			}
			for _, e := range events {
				ts := e.Timestamp.Format(timeFormatShort)
				if e.Field != "" {
					fmt.Printf("[%s] %s %s: %s → %s", ts, e.Type, e.Field, e.OldValue, e.NewValue)
				} else {
					fmt.Printf("[%s] %s", ts, e.Type)
					if e.NewValue != "" {
						fmt.Printf(": %s", e.NewValue)
					}
				}
				if e.Actor != "" {
					fmt.Printf(" (by %s)", e.Actor)
				}
				fmt.Println()
			}
			if len(events) == 0 {
				fmt.Println("No events.")
			}
			return nil
		},
	}
}
