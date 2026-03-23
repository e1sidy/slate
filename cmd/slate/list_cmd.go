package main

import (
	"fmt"

	"github.com/e1sidy/slate"
	"github.com/spf13/cobra"
)

func listCmd() *cobra.Command {
	var status, assignee, taskType, label, parent string
	var priority int
	var showAll, tree bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			p := slate.ListParams{}

			if status != "" {
				s := slate.Status(status)
				p.Status = &s
			}
			if assignee != "" {
				p.Assignee = assignee
			}
			if cmd.Flags().Changed("priority") {
				pr := slate.Priority(priority)
				p.Priority = &pr
			}
			if taskType != "" {
				t := slate.TaskType(taskType)
				p.Type = &t
			}
			if label != "" {
				p.Label = label
			}
			if parent != "" {
				p.ParentID = &parent
			} else if !showAll && !cmd.Flags().Changed("parent") {
				// Default: show root tasks only (no parent)
				empty := ""
				p.ParentID = &empty
			}

			if !showAll {
				p.ExcludeStatuses = []slate.Status{slate.StatusClosed, slate.StatusCancelled}
			}

			tasks, err := store.List(cmd.Context(), p)
			if err != nil {
				return err
			}

			if jsonOutput {
				return printJSON(tasks)
			}

			if tree {
				return printTree(cmd, tasks)
			}

			for _, t := range tasks {
				line := fmt.Sprintf("%s %s %s %s",
					colorStatus(t.Status), colorPriority(t.Priority), t.Title, colorID(t.ID))
				if t.Assignee != "" {
					line += " " + colorAssignee(t.Assignee)
				}
				fmt.Println(line)
			}

			if len(tasks) == 0 {
				fmt.Println("No tasks found.")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Filter by status")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Filter by assignee")
	cmd.Flags().IntVar(&priority, "priority", -1, "Filter by priority")
	cmd.Flags().StringVar(&taskType, "type", "", "Filter by type")
	cmd.Flags().StringVar(&label, "label", "", "Filter by label")
	cmd.Flags().StringVar(&parent, "parent", "", "Filter by parent ID")
	cmd.Flags().BoolVar(&showAll, "all", false, "Include closed/cancelled tasks")
	cmd.Flags().BoolVar(&tree, "tree", false, "Show hierarchical tree view")

	return cmd
}

func printTree(cmd *cobra.Command, tasks []*slate.Task) error {
	for i, t := range tasks {
		isLast := i == len(tasks)-1
		printTreeNode(cmd, t, "", isLast)
	}
	return nil
}

func printTreeNode(cmd *cobra.Command, t *slate.Task, prefix string, isLast bool) {
	connector := "├── "
	if isLast {
		connector = "└── "
	}
	if prefix == "" {
		connector = ""
	}

	line := fmt.Sprintf("%s%s%s %s %s %s",
		prefix, connector, colorStatus(t.Status), colorPriority(t.Priority), t.Title, colorID(t.ID))
	if t.Assignee != "" {
		line += " " + colorAssignee(t.Assignee)
	}
	fmt.Println(line)

	children, err := store.Children(cmd.Context(), t.ID)
	if err != nil || len(children) == 0 {
		return
	}

	childPrefix := prefix
	if prefix != "" {
		if isLast {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}
	}

	for i, child := range children {
		printTreeNode(cmd, child, childPrefix, i == len(children)-1)
	}
}
