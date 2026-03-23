package main

import (
	"fmt"

	"github.com/e1sidy/slate"
	"github.com/spf13/cobra"
)

func attrCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attr",
		Short: "Manage custom attributes",
	}
	cmd.AddCommand(attrDefineCmd(), attrUndefineCmd(), attrSetCmd(), attrGetCmd(), attrDeleteCmd(), attrListCmd())
	return cmd
}

func attrDefineCmd() *cobra.Command {
	var desc string
	cmd := &cobra.Command{
		Use:   "define <key> <type>",
		Short: "Define a custom attribute (string, boolean, object)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := store.DefineAttr(cmd.Context(), args[0], slate.AttrType(args[1]), desc); err != nil {
				return err
			}
			fmt.Printf("Defined attribute: %s (%s)\n", args[0], args[1])
			return nil
		},
	}
	cmd.Flags().StringVar(&desc, "desc", "", "Description")
	return cmd
}

func attrUndefineCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "undefine <key>",
		Short: "Remove attribute definition and all values",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := store.UndefineAttr(cmd.Context(), args[0]); err != nil {
				return err
			}
			fmt.Printf("Undefined attribute: %s\n", args[0])
			return nil
		},
	}
}

func attrSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <task-id> <key> <value>",
		Short: "Set attribute on a task",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := store.SetAttr(cmd.Context(), args[0], args[1], args[2]); err != nil {
				return err
			}
			fmt.Printf("Set %s = %s on %s\n", args[1], args[2], args[0])
			return nil
		},
	}
}

func attrGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <task-id> <key>",
		Short: "Get attribute value from a task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			attr, err := store.GetAttr(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(attr)
			}
			fmt.Printf("%s = %s (%s)\n", attr.Key, attr.Value, attr.Type)
			return nil
		},
	}
}

func attrDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <task-id> <key>",
		Short: "Remove attribute from a task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := store.DeleteAttr(cmd.Context(), args[0], args[1]); err != nil {
				return err
			}
			fmt.Printf("Deleted %s from %s\n", args[1], args[0])
			return nil
		},
	}
}

func attrListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all attribute definitions",
		RunE: func(cmd *cobra.Command, args []string) error {
			defs, err := store.ListAttrDefs(cmd.Context())
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(defs)
			}
			for _, d := range defs {
				fmt.Printf("  %s (%s)", d.Key, d.Type)
				if d.Description != "" {
					fmt.Printf(" — %s", d.Description)
				}
				fmt.Println()
			}
			if len(defs) == 0 {
				fmt.Println("No attributes defined.")
			}
			return nil
		},
	}
}
