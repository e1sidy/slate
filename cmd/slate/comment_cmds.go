package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func commentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment",
		Short: "Manage task comments",
	}
	cmd.AddCommand(commentAddCmd(), commentEditCmd(), commentDeleteCmd(), commentListCmd())
	return cmd
}

func commentAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <task-id> <content>",
		Short: "Add a comment to a task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := store.AddComment(cmd.Context(), args[0], actorName, args[1])
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(c)
			}
			if quietMode {
				fmt.Println(c.ID)
				return nil
			}
			fmt.Printf("Comment added to %s\n", args[0])
			return nil
		},
	}
}

func commentEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit <comment-id> <content>",
		Short: "Edit a comment",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := store.EditComment(cmd.Context(), args[0], args[1]); err != nil {
				return err
			}
			fmt.Printf("Comment %s updated\n", args[0])
			return nil
		},
	}
}

func commentDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <comment-id>",
		Short: "Delete a comment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := store.DeleteComment(cmd.Context(), args[0]); err != nil {
				return err
			}
			fmt.Printf("Comment %s deleted\n", args[0])
			return nil
		},
	}
}

func commentListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <task-id>",
		Short: "List comments for a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			comments, err := store.ListComments(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(comments)
			}
			for _, c := range comments {
				fmt.Printf("[%s] %s: %s\n", c.CreatedAt.Format(timeFormatShort), c.Author, c.Content)
			}
			if len(comments) == 0 {
				fmt.Println("No comments.")
			}
			return nil
		},
	}
}
