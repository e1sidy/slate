package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func exportCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export all data as JSONL",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := os.Stdout
			if file != "" {
				f, err := os.Create(file)
				if err != nil {
					return fmt.Errorf("create file: %w", err)
				}
				defer f.Close()
				w = f
			}
			if err := store.ExportJSONL(cmd.Context(), w); err != nil {
				return err
			}
			if file != "" {
				fmt.Fprintf(os.Stderr, "Exported to %s\n", file)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "Output file (default: stdout)")
	return cmd
}

func importCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import <file>",
		Short: "Import data from JSONL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := os.Open(args[0])
			if err != nil {
				return fmt.Errorf("open file: %w", err)
			}
			defer f.Close()

			if err := store.ImportJSONL(cmd.Context(), f); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Imported from %s\n", args[0])
			return nil
		},
	}
}
