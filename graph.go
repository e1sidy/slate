package slate

import (
	"context"
	"fmt"
	"strings"
)

// DepMermaid generates a Mermaid diagram of the dependency DAG.
// If rootID is non-empty, scopes to that task's subtree; otherwise shows all tasks.
func (s *Store) DepMermaid(ctx context.Context, rootID string) (string, error) {
	var tasks []*Task
	var err error

	if rootID != "" {
		tasks, err = s.Children(ctx, rootID)
		if err != nil {
			return "", fmt.Errorf("get children: %w", err)
		}
		// Include the root task itself.
		root, rerr := s.Get(ctx, rootID)
		if rerr == nil {
			tasks = append([]*Task{root}, tasks...)
		}
	} else {
		tasks, err = s.List(ctx, ListParams{})
		if err != nil {
			return "", fmt.Errorf("list tasks: %w", err)
		}
	}

	if len(tasks) == 0 {
		return "graph TD\n    empty[No tasks]\n", nil
	}

	taskSet := make(map[string]bool)
	for _, t := range tasks {
		taskSet[t.ID] = true
	}

	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Nodes.
	for _, t := range tasks {
		icon := statusIcon(t.Status)
		label := fmt.Sprintf("%s %s", icon, t.Title)
		if t.Estimate > 0 {
			label += fmt.Sprintf(" (%dh)", t.Estimate)
		}
		nodeID := mermaidID(t.ID)
		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", nodeID, escMermaid(label)))

		// Style by status.
		switch t.Status {
		case StatusInProgress:
			sb.WriteString(fmt.Sprintf("    style %s fill:#ffd700\n", nodeID))
		case StatusBlocked:
			sb.WriteString(fmt.Sprintf("    style %s fill:#ff6b6b\n", nodeID))
		case StatusClosed:
			sb.WriteString(fmt.Sprintf("    style %s fill:#69db7c\n", nodeID))
		case StatusCancelled:
			sb.WriteString(fmt.Sprintf("    style %s fill:#adb5bd\n", nodeID))
		}
	}

	// Edges.
	for _, t := range tasks {
		deps, _ := s.ListDependents(ctx, t.ID)
		for _, d := range deps {
			if !taskSet[d.FromID] {
				continue // blocker not in scope
			}
			edgeLabel := string(d.Type)
			sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n",
				mermaidID(d.FromID), edgeLabel, mermaidID(t.ID)))
		}
	}

	return sb.String(), nil
}

// DepDOT generates a Graphviz DOT diagram of the dependency DAG.
func (s *Store) DepDOT(ctx context.Context, rootID string) (string, error) {
	var tasks []*Task
	var err error

	if rootID != "" {
		tasks, err = s.Children(ctx, rootID)
		if err != nil {
			return "", err
		}
		root, rerr := s.Get(ctx, rootID)
		if rerr == nil {
			tasks = append([]*Task{root}, tasks...)
		}
	} else {
		tasks, err = s.List(ctx, ListParams{})
		if err != nil {
			return "", err
		}
	}

	taskSet := make(map[string]bool)
	for _, t := range tasks {
		taskSet[t.ID] = true
	}

	var sb strings.Builder
	sb.WriteString("digraph deps {\n")
	sb.WriteString("    rankdir=TD;\n")
	sb.WriteString("    node [shape=box, style=rounded];\n\n")

	for _, t := range tasks {
		icon := statusIcon(t.Status)
		label := fmt.Sprintf("%s %s %s", icon, t.ID, t.Title)
		color := dotColor(t.Status)
		sb.WriteString(fmt.Sprintf("    \"%s\" [label=\"%s\", fillcolor=\"%s\", style=\"filled,rounded\"];\n",
			t.ID, escDOT(label), color))
	}

	sb.WriteString("\n")
	for _, t := range tasks {
		deps, _ := s.ListDependents(ctx, t.ID)
		for _, d := range deps {
			if !taskSet[d.FromID] {
				continue
			}
			sb.WriteString(fmt.Sprintf("    \"%s\" -> \"%s\" [label=\"%s\"];\n",
				d.FromID, t.ID, d.Type))
		}
	}

	sb.WriteString("}\n")
	return sb.String(), nil
}

func statusIcon(s Status) string {
	switch s {
	case StatusOpen:
		return "[ ]"
	case StatusInProgress:
		return "[>]"
	case StatusBlocked:
		return "[!]"
	case StatusDeferred:
		return "[~]"
	case StatusClosed:
		return "[x]"
	case StatusCancelled:
		return "[-]"
	default:
		return "[?]"
	}
}

func mermaidID(id string) string {
	return strings.ReplaceAll(strings.ReplaceAll(id, "-", "_"), ".", "_")
}

func escMermaid(s string) string {
	s = strings.ReplaceAll(s, "\"", "'")
	return s
}

func escDOT(s string) string {
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

func dotColor(s Status) string {
	switch s {
	case StatusInProgress:
		return "#ffd700"
	case StatusBlocked:
		return "#ff6b6b"
	case StatusClosed:
		return "#69db7c"
	case StatusCancelled:
		return "#adb5bd"
	default:
		return "#ffffff"
	}
}
