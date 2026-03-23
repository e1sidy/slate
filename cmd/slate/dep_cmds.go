package main

import (
	"fmt"

	"github.com/e1sidy/slate"
	"github.com/spf13/cobra"
)

func depCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dep",
		Short: "Manage task dependencies",
	}
	cmd.AddCommand(depAddCmd(), depRemoveCmd(), depListCmd(), depTreeCmd(), depCyclesCmd())
	return cmd
}

func depAddCmd() *cobra.Command {
	var depType string
	cmd := &cobra.Command{
		Use:   "add <from-id> <to-id>",
		Short: "Add dependency (from depends on to)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dt := slate.Blocks
			if depType != "" {
				dt = slate.DepType(depType)
			}
			if err := store.AddDependency(cmd.Context(), args[0], args[1], dt); err != nil {
				return err
			}
			fmt.Printf("Added: %s depends on %s (%s)\n", args[0], args[1], dt)
			return nil
		},
	}
	cmd.Flags().StringVar(&depType, "type", "blocks", "Dependency type (blocks, relates_to, duplicates)")
	return cmd
}

func depRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <from-id> <to-id>",
		Short: "Remove a dependency",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := store.RemoveDependency(cmd.Context(), args[0], args[1]); err != nil {
				return err
			}
			fmt.Printf("Removed dependency: %s -> %s\n", args[0], args[1])
			return nil
		},
	}
}

func depListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <id>",
		Short: "List what a task depends on",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			deps, err := store.ListDependencies(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(deps)
			}
			for _, d := range deps {
				fmt.Printf("  %s (%s)\n", d.ToID, d.Type)
			}
			if len(deps) == 0 {
				fmt.Println("No dependencies.")
			}
			return nil
		},
	}
}

func depTreeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tree <id>",
		Short: "Show dependency tree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tree, err := store.DepTree(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			fmt.Print(tree)
			return nil
		},
	}
}

func depCyclesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cycles",
		Short: "Detect dependency cycles",
		RunE: func(cmd *cobra.Command, args []string) error {
			cycles, err := store.DetectCycles(cmd.Context())
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(cycles)
			}
			if len(cycles) == 0 {
				fmt.Println("No cycles detected.")
				return nil
			}
			for i, cycle := range cycles {
				fmt.Printf("Cycle %d: %s\n", i+1, fmt.Sprint(cycle))
			}
			return nil
		},
	}
}
