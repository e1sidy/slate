package main

import (
	"fmt"
	"os"

	"github.com/e1sidy/slate"
	"github.com/spf13/cobra"
)

func notionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notion",
		Short: "Notion integration commands",
	}
	cmd.AddCommand(
		notionConnectCmd(),
		notionDisconnectCmd(),
		notionStatusCmd(),
		notionSyncCmd(),
		notionConflictsCmd(),
		notionResolveCmd(),
		notionDashboardCmd(),
	)
	return cmd
}

func notionConnectCmd() *cobra.Command {
	var (
		token      string
		databaseID string
		autoInfer  bool
		userID     string
	)

	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Connect to a Notion database",
		Long:  "Validates the token and database access, then saves the connection config.\nUse --auto to auto-detect property mapping from the existing database schema.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if token == "" {
				return fmt.Errorf("--token is required")
			}
			if databaseID == "" {
				return fmt.Errorf("--database-id is required")
			}

			home := slate.DefaultSlateHome()
			var ncfg *slate.NotionConfig

			if autoInfer {
				// Auto-detect property mapping from database schema.
				tempCfg := &slate.NotionConfig{Token: token, DatabaseID: databaseID}
				client := slate.NewNotionClient(tempCfg)

				fmt.Fprintln(os.Stderr, "Inferring property mapping from database schema...")
				inferred, err := slate.InferMapping(cmd.Context(), client.API, databaseID)
				if err != nil {
					return fmt.Errorf("infer mapping: %w", err)
				}
				inferred.Token = token
				ncfg = inferred
			} else {
				ncfg = &slate.NotionConfig{
					Token:      token,
					DatabaseID: databaseID,
				}
				// Merge defaults for unmapped fields.
				defaults := slate.DefaultNotionConfig()
				ncfg.RateLimit = defaults.RateLimit
				ncfg.PropertyMap = defaults.PropertyMap
				ncfg.DepMap = defaults.DepMap
				ncfg.StatusMap = defaults.StatusMap
				ncfg.PriorityMap = defaults.PriorityMap
				ncfg.PriorityReverse = defaults.PriorityReverse
				ncfg.AutoCreateProperties = defaults.AutoCreateProperties
			}

			// Validate connection.
			client := slate.NewNotionClient(ncfg)
			fmt.Fprintln(os.Stderr, "Validating connection...")
			if err := client.Ping(cmd.Context()); err != nil {
				return fmt.Errorf("connection failed: %w", err)
			}

			// Set user ID if provided.
			if userID != "" {
				ncfg.UserID = userID
			} else if len(client.Users) > 0 {
				fmt.Fprintln(os.Stderr, "\nAvailable users (use --user-id to filter syncs):")
				for _, u := range client.Users {
					fmt.Fprintf(os.Stderr, "  %s  %s\n", u.ID, u.Name)
				}
				fmt.Fprintln(os.Stderr)
			}

			// Save config.
			if err := slate.SaveNotionConfig(home, ncfg); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Connected to Notion database %s\n", databaseID)
			fmt.Fprintf(os.Stderr, "Config saved to %s\n", slate.NotionConfigPath(home))

			if ncfg.UserID != "" {
				fmt.Fprintf(os.Stderr, "Sync filtered to user: %s\n", ncfg.UserID)
			}
			if len(client.Users) > 0 {
				fmt.Fprintf(os.Stderr, "Cached %d workspace users for assignee mapping\n", len(client.Users))
			}

			if jsonOutput {
				return printJSON(map[string]any{
					"database_id":  databaseID,
					"property_map": ncfg.PropertyMap,
					"status_map":   ncfg.StatusMap,
					"dep_map":      ncfg.DepMap,
					"users":        len(client.Users),
				})
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&token, "token", "", "Notion API token (required)")
	cmd.Flags().StringVar(&databaseID, "database-id", "", "Notion database ID (required)")
	cmd.Flags().BoolVar(&autoInfer, "auto", false, "Auto-detect property mapping from database schema")
	cmd.Flags().StringVar(&userID, "user-id", "", "Notion user ID to filter syncs (only sync this user's tasks)")
	return cmd
}

func notionDisconnectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disconnect",
		Short: "Disconnect from Notion",
		RunE: func(cmd *cobra.Command, args []string) error {
			home := slate.DefaultSlateHome()
			if err := slate.DeleteNotionConfig(home); err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr, "Disconnected from Notion")
			return nil
		},
	}
}

func notionStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Notion connection status and sync stats",
		RunE: func(cmd *cobra.Command, args []string) error {
			home := slate.DefaultSlateHome()
			ncfg, err := slate.LoadNotionConfig(home)
			if err != nil {
				return err
			}
			if ncfg == nil {
				fmt.Println("Not connected to Notion")
				fmt.Println("Run: slate notion connect --token <token> --database-id <id>")
				return nil
			}

			// Get sync stats from store.
			records, err := store.ListSyncRecords(cmd.Context())
			if err != nil {
				return fmt.Errorf("list sync records: %w", err)
			}
			conflicts, err := store.ListConflicts(cmd.Context())
			if err != nil {
				return fmt.Errorf("list conflicts: %w", err)
			}

			if jsonOutput {
				return printJSON(map[string]any{
					"connected":    true,
					"database_id":  ncfg.DatabaseID,
					"property_map": ncfg.PropertyMap,
					"status_map":   ncfg.StatusMap,
					"dep_map":      ncfg.DepMap,
					"synced_tasks": len(records),
					"conflicts":    len(conflicts),
				})
			}

			fmt.Printf("Connected: yes\n")
			fmt.Printf("Database:  %s\n", ncfg.DatabaseID)
			fmt.Printf("Rate limit: %s\n", ncfg.RateLimit)
			fmt.Printf("Auto-create: %t\n", ncfg.AutoCreateProperties)
			if ncfg.UserID != "" {
				fmt.Printf("User ID:   %s (sync filtered to this user)\n", ncfg.UserID)
			} else {
				fmt.Printf("User ID:   (not set — syncs all users)\n")
			}
			fmt.Println()

			fmt.Println("Property Mapping:")
			fmt.Printf("  title      → %s\n", ncfg.PropertyMap.Title)
			fmt.Printf("  status     → %s\n", ncfg.PropertyMap.Status)
			fmt.Printf("  priority   → %s\n", ncfg.PropertyMap.Priority)
			fmt.Printf("  assignee   → %s\n", ncfg.PropertyMap.Assignee)
			fmt.Printf("  labels     → %s\n", ncfg.PropertyMap.Labels)
			fmt.Printf("  due_at     → %s\n", ncfg.PropertyMap.DueAt)
			fmt.Printf("  parent_id  → %s\n", ncfg.PropertyMap.ParentID)
			if ncfg.PropertyMap.Type != "" {
				fmt.Printf("  type       → %s\n", ncfg.PropertyMap.Type)
			}
			if ncfg.PropertyMap.Progress != "" {
				fmt.Printf("  progress   → %s\n", ncfg.PropertyMap.Progress)
			}
			fmt.Println()

			if len(ncfg.DepMap) > 0 {
				fmt.Println("Dependency Mapping:")
				for slateType, notionProp := range ncfg.DepMap {
					fmt.Printf("  %s → %s\n", slateType, notionProp)
				}
				fmt.Println()
			}

			fmt.Printf("Synced tasks: %d\n", len(records))
			fmt.Printf("Conflicts:    %d\n", len(conflicts))

			return nil
		},
	}
}

func notionSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync tasks between Slate and Notion",
	}
	cmd.AddCommand(
		notionSyncPushCmd(),
		notionSyncPullCmd(),
		notionSyncBidirCmd(),
	)
	return cmd
}

func notionSyncPushCmd() *cobra.Command {
	var (
		taskID     string
		filterStr  string
	)

	cmd := &cobra.Command{
		Use:   "push",
		Short: "Push Slate tasks to Notion",
		Long:  "Creates or updates Notion pages from Slate tasks.\nUse --task for a single task or --filter for selective sync.",
		RunE: func(cmd *cobra.Command, args []string) error {
			home := slate.DefaultSlateHome()
			ncfg, err := slate.LoadNotionConfig(home)
			if err != nil {
				return err
			}
			if ncfg == nil {
				return fmt.Errorf("not connected to Notion. Run: slate notion connect")
			}

			client := slate.NewNotionClient(ncfg)

			// Ensure properties exist.
			created, warnings, err := client.EnsureProperties(cmd.Context())
			if err != nil {
				return fmt.Errorf("ensure properties: %w", err)
			}
			for _, c := range created {
				fmt.Fprintf(os.Stderr, "Created Notion property: %s\n", c)
			}
			for _, w := range warnings {
				fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
			}

			if taskID != "" {
				// Push single task.
				if err := client.PushTask(cmd.Context(), store, taskID); err != nil {
					return err
				}
				if !quietMode {
					fmt.Fprintf(os.Stderr, "Pushed %s to Notion\n", taskID)
				}
				return nil
			}

			// Push all matching tasks.
			// Default: sync root tasks only (no parent_id). Use --filter to override.
			filter := slate.ListParams{}
			if filterStr != "" {
				filter = parseNotionFilter(filterStr)
			} else {
				// Root tasks only: ParentID = pointer to empty string.
				root := ""
				filter.ParentID = &root
			}

			result, err := client.PushAll(cmd.Context(), store, filter)
			if err != nil {
				return err
			}

			if jsonOutput {
				return printJSON(result)
			}

			fmt.Fprintf(os.Stderr, "Push complete: %d created, %d updated, %d skipped\n",
				result.Created, result.Updated, result.Skipped)
			for _, e := range result.Errors {
				fmt.Fprintf(os.Stderr, "  Error: %s\n", e)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&taskID, "task", "", "Push a single task by ID")
	cmd.Flags().StringVar(&filterStr, "filter", "", "Filter tasks (e.g. \"type:epic\", \"status:open\")")
	return cmd
}

func notionSyncPullCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pull",
		Short: "Pull Notion changes to Slate",
		Long:  "Detects pages modified since last sync and applies changes to local Slate tasks.\nAlso creates new Slate tasks from unsynced Notion pages.",
		RunE: func(cmd *cobra.Command, args []string) error {
			home := slate.DefaultSlateHome()
			ncfg, err := slate.LoadNotionConfig(home)
			if err != nil {
				return err
			}
			if ncfg == nil {
				return fmt.Errorf("not connected to Notion. Run: slate notion connect")
			}

			client := slate.NewNotionClient(ncfg)

			result, err := client.PullChanges(cmd.Context(), store)
			if err != nil {
				return err
			}

			if jsonOutput {
				return printJSON(result)
			}

			fmt.Fprintf(os.Stderr, "Pull complete: %d updated, %d created, %d comments, %d skipped\n",
				result.Updated, result.Created, result.Comments, result.Skipped)
			for _, e := range result.Errors {
				fmt.Fprintf(os.Stderr, "  Error: %s\n", e)
			}
			return nil
		},
	}
}

// parseNotionFilter parses a simple filter string into ListParams.
// Format: "key:value" pairs, e.g. "type:epic", "status:open".
func parseNotionFilter(s string) slate.ListParams {
	params := slate.ListParams{}
	// Simple key:value parsing.
	for _, part := range splitFilter(s) {
		if len(part) < 3 {
			continue
		}
		for i, c := range part {
			if c == ':' {
				key := part[:i]
				val := part[i+1:]
				switch key {
				case "type":
					tp := slate.TaskType(val)
					params.Type = &tp
				case "status":
					st := slate.Status(val)
					params.Status = &st
				case "assignee":
					params.Assignee = val
				case "priority":
					// Parse priority number.
					if len(val) == 1 && val[0] >= '0' && val[0] <= '4' {
						p := slate.Priority(val[0] - '0')
						params.Priority = &p
					}
				}
				break
			}
		}
	}
	return params
}

// splitFilter splits a filter string by spaces or " OR ".
func splitFilter(s string) []string {
	var parts []string
	current := ""
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(s[i])
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	// Filter out "OR".
	var filtered []string
	for _, p := range parts {
		if p != "OR" && p != "or" {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func notionSyncBidirCmd() *cobra.Command {
	var filterStr string

	cmd := &cobra.Command{
		Use:   "all",
		Short: "Bidirectional sync between Slate and Notion",
		Long:  "Pushes local changes, pulls remote changes, and detects conflicts.\nConflicts are auto-resolved with last-write-wins by default.",
		RunE: func(cmd *cobra.Command, args []string) error {
			home := slate.DefaultSlateHome()
			ncfg, err := slate.LoadNotionConfig(home)
			if err != nil {
				return err
			}
			if ncfg == nil {
				return fmt.Errorf("not connected to Notion. Run: slate notion connect")
			}

			client := slate.NewNotionClient(ncfg)

			filter := slate.ListParams{}
			if filterStr != "" {
				filter = parseNotionFilter(filterStr)
			}

			result, err := client.Sync(cmd.Context(), store, filter)
			if err != nil {
				return err
			}

			if jsonOutput {
				return printJSON(result)
			}

			fmt.Fprintf(os.Stderr, "Sync complete:\n")
			fmt.Fprintf(os.Stderr, "  Push: %d created, %d updated\n", result.Pushed.Created, result.Pushed.Updated)
			fmt.Fprintf(os.Stderr, "  Pull: %d updated, %d created, %d comments\n",
				result.Pulled.Updated, result.Pulled.Created, result.Pulled.Comments)
			if len(result.Conflicts) > 0 {
				fmt.Fprintf(os.Stderr, "  Conflicts: %d (auto-resolved with last-write-wins)\n", len(result.Conflicts))
				for _, c := range result.Conflicts {
					fmt.Fprintf(os.Stderr, "    %s.%s: local=%q notion=%q → %s\n",
						c.TaskID, c.Field, c.SlateValue, c.NotionValue, c.Resolution)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&filterStr, "filter", "", "Filter tasks (e.g. \"type:epic\")")
	return cmd
}

func notionConflictsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "conflicts",
		Short: "List unresolved Notion sync conflicts",
		RunE: func(cmd *cobra.Command, args []string) error {
			conflicts, err := store.ListConflicts(cmd.Context())
			if err != nil {
				return err
			}

			if jsonOutput {
				return printJSON(conflicts)
			}

			if len(conflicts) == 0 {
				fmt.Println("No conflicts")
				return nil
			}

			for _, c := range conflicts {
				fmt.Printf("%s (page %s): %s\n", c.TaskID, c.NotionPageID, c.ConflictStatus)
			}
			return nil
		},
	}
}

func notionResolveCmd() *cobra.Command {
	var prefer string

	cmd := &cobra.Command{
		Use:   "resolve <task-id>",
		Short: "Resolve a Notion sync conflict",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if prefer != "local" && prefer != "notion" {
				return fmt.Errorf("--prefer must be 'local' or 'notion'")
			}

			home := slate.DefaultSlateHome()
			ncfg, err := slate.LoadNotionConfig(home)
			if err != nil {
				return err
			}
			if ncfg == nil {
				return fmt.Errorf("not connected to Notion")
			}

			client := slate.NewNotionClient(ncfg)
			if err := client.ResolveConflict(cmd.Context(), store, args[0], prefer); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Resolved conflict for %s (preferred: %s)\n", args[0], prefer)
			return nil
		},
	}
	cmd.Flags().StringVar(&prefer, "prefer", "", "Resolution preference: 'local' or 'notion' (required)")
	return cmd
}

func notionDashboardCmd() *cobra.Command {
	var weekly bool

	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Push metrics dashboard to Notion",
		RunE: func(cmd *cobra.Command, args []string) error {
			home := slate.DefaultSlateHome()
			ncfg, err := slate.LoadNotionConfig(home)
			if err != nil {
				return err
			}
			if ncfg == nil {
				return fmt.Errorf("not connected to Notion. Run: slate notion connect")
			}

			client := slate.NewNotionClient(ncfg)

			if weekly {
				pageID, err := client.PushWeeklyDigest(cmd.Context(), store)
				if err != nil {
					return err
				}
				if jsonOutput {
					return printJSON(map[string]string{"page_id": pageID})
				}
				fmt.Fprintf(os.Stderr, "Weekly digest created: %s\n", pageID)
				return nil
			}

			pageID, err := client.PushDashboard(cmd.Context(), store)
			if err != nil {
				return err
			}

			// Save dashboard page ID if new.
			if ncfg.DashboardPageID == "" {
				ncfg.DashboardPageID = pageID
				slate.SaveNotionConfig(home, ncfg)
			}

			if jsonOutput {
				return printJSON(map[string]string{"page_id": pageID})
			}
			fmt.Fprintf(os.Stderr, "Dashboard updated: %s\n", pageID)
			return nil
		},
	}

	cmd.Flags().BoolVar(&weekly, "weekly", false, "Create weekly digest instead of dashboard")
	return cmd
}
