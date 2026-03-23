package main

import (
	"context"
	"fmt"
	"os"
	"time"

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
	timeoutStr string
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
			// Apply timeout to context if set.
			if timeoutStr != "" {
				dur, err := time.ParseDuration(timeoutStr)
				if err != nil {
					return fmt.Errorf("invalid timeout %q: %w", timeoutStr, err)
				}
				ctx, cancel := context.WithTimeout(cmd.Context(), dur)
				_ = cancel // context will be cancelled when deadline expires
				cmd.SetContext(ctx)
			}

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
	cmd.PersistentFlags().StringVar(&timeoutStr, "timeout", "", "Operation timeout (e.g. 30s, 5m)")

	// Task management.
	cmd.AddCommand(createCmd())
	cmd.AddCommand(showCmd())
	cmd.AddCommand(updateCmd())
	cmd.AddCommand(closeCmd())
	cmd.AddCommand(cancelCmd())
	cmd.AddCommand(reopenCmd())
	cmd.AddCommand(deleteCmd())
	cmd.AddCommand(searchCmd())

	// Querying.
	cmd.AddCommand(listCmd())
	cmd.AddCommand(readyCmd())
	cmd.AddCommand(blockedCmd())
	cmd.AddCommand(childrenCmd())
	cmd.AddCommand(eventsCmd())
	cmd.AddCommand(nextCmd())

	// Dependencies.
	cmd.AddCommand(depCmd())

	// Attributes.
	cmd.AddCommand(attrCmd())

	// Comments.
	cmd.AddCommand(commentCmd())

	// Checkpoints.
	cmd.AddCommand(checkpointCmd())

	// Sync.
	cmd.AddCommand(exportCmd())
	cmd.AddCommand(importCmd())

	// Config.
	cmd.AddCommand(configCmd())

	// Metrics.
	cmd.AddCommand(metricsCmd())

	// Utilities.
	cmd.AddCommand(statsCmd())
	cmd.AddCommand(doctorCmd())
	cmd.AddCommand(versionCmd())
	cmd.AddCommand(completionCmd())

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
