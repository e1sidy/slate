package main

import (
	"fmt"

	"github.com/e1sidy/slate"
	"github.com/spf13/cobra"
)

func checkpointCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "checkpoint",
		Short: "Manage task checkpoints",
	}
	cmd.AddCommand(checkpointAddCmd(), checkpointListCmd())
	return cmd
}

func checkpointAddCmd() *cobra.Command {
	var done, decisions, next, blockers string
	var files []string

	cmd := &cobra.Command{
		Use:   "add <task-id>",
		Short: "Add a progress checkpoint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cp, err := store.AddCheckpoint(cmd.Context(), args[0], actorName, slate.CheckpointParams{
				Done:      done,
				Decisions: decisions,
				Next:      next,
				Blockers:  blockers,
				Files:     files,
			})
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(cp)
			}
			if quietMode {
				fmt.Println(cp.ID)
				return nil
			}
			fmt.Printf("Checkpoint added to %s\n", args[0])
			return nil
		},
	}

	cmd.Flags().StringVar(&done, "done", "", "What was accomplished (required)")
	cmd.Flags().StringVar(&decisions, "decisions", "", "Key decisions")
	cmd.Flags().StringVar(&next, "next", "", "What should happen next")
	cmd.Flags().StringVar(&blockers, "blockers", "", "Current blockers")
	cmd.Flags().StringSliceVar(&files, "files", nil, "Files touched")
	cmd.MarkFlagRequired("done")

	return cmd
}

func checkpointListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <task-id>",
		Short: "List checkpoints for a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cps, err := store.ListCheckpoints(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(cps)
			}
			for _, cp := range cps {
				fmt.Printf("[%s] Done: %s\n", cp.CreatedAt.Format(timeFormatShort), cp.Done)
				if cp.Decisions != "" {
					fmt.Printf("  Decisions: %s\n", cp.Decisions)
				}
				if cp.Next != "" {
					fmt.Printf("  Next: %s\n", cp.Next)
				}
				if cp.Blockers != "" {
					fmt.Printf("  Blockers: %s\n", cp.Blockers)
				}
			}
			if len(cps) == 0 {
				fmt.Println("No checkpoints.")
			}
			return nil
		},
	}
}
