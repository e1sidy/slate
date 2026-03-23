package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/e1sidy/slate"
	"github.com/spf13/cobra"
)

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and manage configuration",
	}
	cmd.AddCommand(configShowCmd(), configSetCmd(), configGetCmd(), initConfigCmd())
	return cmd
}

func configShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := slate.LoadConfig("")
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(c)
			}
			fmt.Printf("Prefix:       %s\n", c.Prefix)
			fmt.Printf("DB Path:      %s\n", c.DBPath)
			fmt.Printf("Hash Length:  %d\n", c.HashLen)
			fmt.Printf("Default View: %s\n", c.DefaultView)
			fmt.Printf("Show All:     %t\n", c.ShowAll)
			fmt.Printf("Lease Timeout: %s\n", c.LeaseTimeout)
			fmt.Printf("Home:         %s\n", slate.DefaultSlateHome())
			return nil
		},
	}
}

func configSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: `Valid keys:
  prefix        ID prefix (string)
  hash_length   ID hash length (3-8)
  default_view  Default list view (list, tree)
  show_all      Show closed tasks by default (true, false)`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := slate.LoadConfig("")
			if err != nil {
				return err
			}

			key, val := args[0], args[1]
			switch key {
			case "prefix":
				c.Prefix = val
			case "hash_length":
				n, err := strconv.Atoi(val)
				if err != nil || n < 3 || n > 8 {
					return fmt.Errorf("hash_length must be 3-8")
				}
				c.HashLen = n
			case "default_view":
				if val != "list" && val != "tree" {
					return fmt.Errorf("default_view must be 'list' or 'tree'")
				}
				c.DefaultView = val
			case "show_all":
				c.ShowAll = val == "true"
			default:
				return fmt.Errorf("unknown config key: %s", key)
			}

			if err := slate.SaveConfig("", c); err != nil {
				return err
			}
			fmt.Printf("Set %s = %s\n", key, val)
			return nil
		},
	}
}

func configGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := slate.LoadConfig("")
			if err != nil {
				return err
			}

			switch args[0] {
			case "prefix":
				fmt.Println(c.Prefix)
			case "hash_length":
				fmt.Println(c.HashLen)
			case "default_view":
				fmt.Println(c.DefaultView)
			case "show_all":
				fmt.Println(c.ShowAll)
			case "db_path":
				fmt.Println(c.DBPath)
			case "home":
				fmt.Println(slate.DefaultSlateHome())
			default:
				return fmt.Errorf("unknown config key: %s", args[0])
			}
			return nil
		},
	}
}

func initConfigCmd() *cobra.Command {
	var prefix string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize Slate home directory and config",
		RunE: func(cmd *cobra.Command, args []string) error {
			home := slate.DefaultSlateHome()
			if err := os.MkdirAll(home, 0o755); err != nil {
				return fmt.Errorf("create home: %w", err)
			}

			configPath := filepath.Join(home, "slate.yaml")
			if _, err := os.Stat(configPath); err == nil {
				fmt.Fprintf(os.Stderr, "Config already exists at %s\n", configPath)
				return nil
			}

			c := slate.DefaultConfig()
			if prefix != "" {
				c.Prefix = prefix
			}

			if err := slate.SaveConfig(configPath, &c); err != nil {
				return err
			}
			fmt.Printf("Initialized Slate at %s\n", home)
			fmt.Printf("  Config: %s\n", configPath)
			fmt.Printf("  DB:     %s\n", c.DBPath)
			fmt.Printf("  Prefix: %s\n", c.Prefix)
			return nil
		},
	}
	cmd.Flags().StringVar(&prefix, "prefix", "", "ID prefix (default: st)")
	return cmd
}
