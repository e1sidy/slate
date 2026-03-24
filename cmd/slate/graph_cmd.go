package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func graphCmd() *cobra.Command {
	var (
		taskID string
		output string
	)

	cmd := &cobra.Command{
		Use:   "graph",
		Short: "Generate dependency graph visualization",
		Long:  "Generates a Mermaid or Graphviz DOT diagram of the dependency DAG.",
		RunE: func(cmd *cobra.Command, args []string) error {
			var result string
			var err error

			switch output {
			case "mermaid", "":
				result, err = store.DepMermaid(cmd.Context(), taskID)
			case "dot":
				result, err = store.DepDOT(cmd.Context(), taskID)
			default:
				return fmt.Errorf("unsupported output format: %q (use 'mermaid' or 'dot')", output)
			}

			if err != nil {
				return err
			}
			fmt.Print(result)
			return nil
		},
	}

	cmd.Flags().StringVar(&taskID, "task", "", "Scope to a specific task's subtree")
	cmd.Flags().StringVar(&output, "output", "mermaid", "Output format: mermaid, dot")
	return cmd
}
