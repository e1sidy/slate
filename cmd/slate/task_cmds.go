package main

import (
	"fmt"
	"strings"

	"github.com/e1sidy/slate"
	"github.com/spf13/cobra"
)

func createCmd() *cobra.Command {
	var desc, taskType, assignee, notes, labels, parent, createdBy, metadata string
	var priority int

	cmd := &cobra.Command{
		Use:   "create <title>",
		Short: "Create a new task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := slate.CreateParams{
				Title:       args[0],
				Description: desc,
				Priority:    slate.Priority(priority),
				Assignee:    assignee,
				Notes:       notes,
				CreatedBy:   createdBy,
				Metadata:    metadata,
				ParentID:    parent,
			}
			if taskType != "" {
				p.Type = slate.TaskType(taskType)
			}
			if labels != "" {
				p.Labels = strings.Split(labels, ",")
			}

			task, err := store.Create(cmd.Context(), p)
			if err != nil {
				return err
			}

			if jsonOutput {
				return printJSON(task)
			}
			if quietMode {
				fmt.Println(task.ID)
				return nil
			}
			fmt.Printf("Created %s: %s\n", task.ID, task.Title)
			return nil
		},
	}

	cmd.Flags().StringVar(&desc, "desc", "", "Task description")
	cmd.Flags().StringVar(&taskType, "type", "", "Task type (task, bug, feature, epic, chore)")
	cmd.Flags().IntVar(&priority, "priority", 2, "Priority (0=critical, 4=backlog)")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Assignee")
	cmd.Flags().StringVar(&notes, "notes", "", "Notes")
	cmd.Flags().StringVar(&labels, "labels", "", "Labels (comma-separated)")
	cmd.Flags().StringVar(&parent, "parent", "", "Parent task ID")
	cmd.Flags().StringVar(&createdBy, "created-by", "", "Created by")
	cmd.Flags().StringVar(&metadata, "metadata", "", "Metadata JSON")

	return cmd
}

func showCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show task details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			task, err := store.GetFull(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			if jsonOutput {
				return printJSON(task)
			}

			fmt.Printf("%s %s %s %s\n", colorStatus(task.Status), colorPriority(task.Priority), task.Title, colorID(task.ID))
			if task.Description != "" {
				fmt.Printf("  Description: %s\n", task.Description)
			}
			fmt.Printf("  Type: %s\n", task.Type)
			if task.Assignee != "" {
				fmt.Printf("  Assignee: %s\n", colorAssignee(task.Assignee))
			}
			if task.ParentID != "" {
				fmt.Printf("  Parent: %s\n", task.ParentID)
			}
			if len(task.Labels) > 0 {
				fmt.Printf("  Labels: %s\n", strings.Join(task.Labels, ", "))
			}
			if task.CloseReason != "" {
				fmt.Printf("  Close reason: %s\n", task.CloseReason)
			}
			if len(task.Attrs) > 0 {
				fmt.Println("  Attributes:")
				for k, v := range task.Attrs {
					fmt.Printf("    %s = %s\n", k, v)
				}
			}
			fmt.Printf("  Created: %s\n", task.CreatedAt.Format(timeFormatFull))
			fmt.Printf("  Updated: %s\n", task.UpdatedAt.Format(timeFormatFull))

			// Show latest checkpoint.
			cp, err := store.LatestCheckpoint(cmd.Context(), task.ID)
			if err == nil && cp != nil {
				fmt.Printf("\n  Latest checkpoint:\n")
				fmt.Printf("    Done: %s\n", cp.Done)
				if cp.Decisions != "" {
					fmt.Printf("    Decisions: %s\n", cp.Decisions)
				}
				if cp.Next != "" {
					fmt.Printf("    Next: %s\n", cp.Next)
				}
			}

			// Show comments.
			comments, _ := store.ListComments(cmd.Context(), task.ID)
			if len(comments) > 0 {
				fmt.Printf("\n  Comments (%d):\n", len(comments))
				for _, c := range comments {
					fmt.Printf("    [%s] %s: %s\n", c.CreatedAt.Format(timeFormatShort), c.Author, c.Content)
				}
			}

			// Show dependencies.
			deps, _ := store.ListDependencies(cmd.Context(), task.ID)
			if len(deps) > 0 {
				fmt.Printf("\n  Depends on:\n")
				for _, d := range deps {
					fmt.Printf("    %s (%s)\n", d.ToID, d.Type)
				}
			}

			return nil
		},
	}
}

func updateCmd() *cobra.Command {
	var title, desc, status, assignee, notes, labels, parent string
	var priority int
	var claim, orphan bool

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]

			if claim {
				if assignee == "" {
					assignee = actorName
				}
				result, err := store.Claim(cmd.Context(), id, assignee)
				if err != nil {
					return err
				}
				fmt.Printf("Claimed %s by %s\n", id, assignee)
				if result.ParentProgressed {
					fmt.Printf("Parent %s → in_progress\n", result.ParentID)
				}
				return nil
			}

			if status != "" {
				if err := store.UpdateStatus(cmd.Context(), id, slate.Status(status), actorName); err != nil {
					return err
				}
				fmt.Printf("Status of %s set to %s\n", id, status)
				return nil
			}

			params := slate.UpdateParams{}
			if cmd.Flags().Changed("title") {
				params.Title = &title
			}
			if cmd.Flags().Changed("desc") {
				params.Description = &desc
			}
			if cmd.Flags().Changed("priority") {
				p := slate.Priority(priority)
				params.Priority = &p
			}
			if cmd.Flags().Changed("assignee") {
				params.Assignee = &assignee
			}
			if cmd.Flags().Changed("notes") {
				params.Notes = &notes
			}
			if cmd.Flags().Changed("labels") {
				l := strings.Split(labels, ",")
				params.Labels = &l
			}
			if cmd.Flags().Changed("parent") {
				params.ParentID = &parent
			}
			if orphan {
				params.Orphan = true
			}

			task, err := store.Update(cmd.Context(), id, params, actorName)
			if err != nil {
				return err
			}

			if jsonOutput {
				return printJSON(task)
			}
			fmt.Printf("Updated %s: %s\n", task.ID, task.Title)
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "New title")
	cmd.Flags().StringVar(&desc, "desc", "", "New description")
	cmd.Flags().StringVar(&status, "status", "", "New status")
	cmd.Flags().IntVar(&priority, "priority", -1, "New priority")
	cmd.Flags().StringVar(&assignee, "assignee", "", "New assignee")
	cmd.Flags().StringVar(&notes, "notes", "", "New notes")
	cmd.Flags().StringVar(&labels, "labels", "", "New labels (comma-separated)")
	cmd.Flags().StringVar(&parent, "parent", "", "Set parent task ID")
	cmd.Flags().BoolVar(&orphan, "orphan", false, "Remove parent")
	cmd.Flags().BoolVar(&claim, "claim", false, "Claim task (set assignee + in_progress)")
	cmd.MarkFlagsMutuallyExclusive("parent", "orphan")

	return cmd
}

func closeCmd() *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   "close <id>",
		Short: "Close a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := store.CloseTask(cmd.Context(), args[0], reason, actorName); err != nil {
				return err
			}
			fmt.Printf("Closed %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "Close reason")
	return cmd
}

func cancelCmd() *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   "cancel <id>",
		Short: "Cancel a task (cascades to children)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := store.CancelTask(cmd.Context(), args[0], reason, actorName); err != nil {
				return err
			}
			fmt.Printf("Cancelled %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "Cancel reason")
	return cmd
}

func reopenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reopen <id>",
		Short: "Reopen a closed or cancelled task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := store.Reopen(cmd.Context(), args[0], actorName); err != nil {
				return err
			}
			fmt.Printf("Reopened %s\n", args[0])
			return nil
		},
	}
}

func deleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Permanently delete a task and its children",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := store.DeleteTask(cmd.Context(), args[0], actorName); err != nil {
				return err
			}
			fmt.Printf("Deleted %s\n", args[0])
			return nil
		},
	}
}

func searchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search tasks by title or description",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tasks, err := store.Search(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(tasks)
			}
			for _, t := range tasks {
				fmt.Printf("%s %s %s %s\n", colorStatus(t.Status), colorPriority(t.Priority), t.Title, colorID(t.ID))
			}
			if len(tasks) == 0 {
				fmt.Println("No tasks found.")
			}
			return nil
		},
	}
}
