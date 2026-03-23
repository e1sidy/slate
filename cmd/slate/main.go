package main

import (
	"fmt"
	"os"

	"github.com/e1sidy/slate"
	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags.
var Version = "dev"

var (
	store      *slate.Store
	cfg        *slate.Config
	jsonOutput bool
	quietMode  bool
	actorName  string
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "slate",
		Short: "Lightweight task management CLI backed by SQLite",
		Long:  "Slate is a task management tool with dependencies, hierarchy, events, and an embeddable Go SDK.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip store setup for commands that don't need it.
			name := cmd.Name()
			if name == "version" || name == "completion" {
				return nil
			}
			// Also skip for the root command itself (shows help).
			if cmd.Parent() == nil || !cmd.HasParent() {
				return nil
			}

			var err error
			cfg, err = slate.LoadConfig("")
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			store, err = slate.Open(cmd.Context(), cfg.DBPath,
				slate.WithPrefix(cfg.Prefix),
				slate.WithHashLength(cfg.HashLen),
				slate.WithConfig(cfg),
			)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			return nil
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			if store != nil {
				store.Close()
			}
		},
	}

	cmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.PersistentFlags().BoolVar(&quietMode, "quiet", false, "Minimal output (just IDs)")
	cmd.PersistentFlags().StringVar(&actorName, "actor", "cli", "Actor name for event attribution")

	cmd.AddCommand(versionCmd())

	return cmd
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("slate %s\n", Version)
		},
	}
}
