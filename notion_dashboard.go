package slate

import (
	"context"
	"fmt"
	"time"

	"github.com/jomei/notionapi"
)

// PushDashboard creates or updates a Notion page with current metrics.
func (nc *NotionClient) PushDashboard(ctx context.Context, store *Store) (string, error) {
	now := time.Now().UTC()
	weekAgo := now.AddDate(0, 0, -7)

	params := MetricsParams{
		From: &weekAgo,
		To:   &now,
	}
	report, err := store.Metrics(ctx, params)
	if err != nil {
		return "", fmt.Errorf("get metrics: %w", err)
	}

	blocks := buildDashboardBlocks(ctx, store, report, now)

	pageID := nc.Config.DashboardPageID
	if pageID != "" {
		// Update existing dashboard page.
		nc.rateLimit()
		_, err := nc.API.UpdatePage(ctx, notionapi.PageID(pageID), &notionapi.PageUpdateRequest{
			Properties: notionapi.Properties{
				"title": notionapi.TitleProperty{
					Type: notionapi.PropertyTypeTitle,
					Title: []notionapi.RichText{
						{Type: "text", Text: &notionapi.Text{Content: fmt.Sprintf("Slate Dashboard — %s", now.Format("2006-01-02 15:04"))}},
					},
				},
			},
		})
		if err != nil {
			return "", fmt.Errorf("update dashboard page: %w", err)
		}

		// Replace page content.
		nc.rateLimit()
		_, err = nc.API.AppendBlockChildren(ctx, notionapi.BlockID(pageID), &notionapi.AppendBlockChildrenRequest{
			Children: blocks,
		})
		if err != nil {
			return "", fmt.Errorf("update dashboard blocks: %w", err)
		}

		return pageID, nil
	}

	// Create new dashboard page.
	nc.rateLimit()
	page, err := nc.API.CreatePage(ctx, &notionapi.PageCreateRequest{
		Parent: notionapi.Parent{
			Type:       notionapi.ParentTypeDatabaseID,
			DatabaseID: notionapi.DatabaseID(nc.Config.DatabaseID),
		},
		Properties: notionapi.Properties{
			"title": notionapi.TitleProperty{
				Type: notionapi.PropertyTypeTitle,
				Title: []notionapi.RichText{
					{Type: "text", Text: &notionapi.Text{Content: fmt.Sprintf("Slate Dashboard — %s", now.Format("2006-01-02 15:04"))}},
				},
			},
		},
		Children: blocks,
	})
	if err != nil {
		return "", fmt.Errorf("create dashboard page: %w", err)
	}

	return string(page.ID), nil
}

// PushWeeklyDigest creates a new Notion page with a weekly summary.
func (nc *NotionClient) PushWeeklyDigest(ctx context.Context, store *Store) (string, error) {
	now := time.Now().UTC()
	weekAgo := now.AddDate(0, 0, -7)

	blocks := buildWeeklyBlocks(ctx, store, weekAgo, now)

	nc.rateLimit()
	page, err := nc.API.CreatePage(ctx, &notionapi.PageCreateRequest{
		Parent: notionapi.Parent{
			Type:       notionapi.ParentTypeDatabaseID,
			DatabaseID: notionapi.DatabaseID(nc.Config.DatabaseID),
		},
		Properties: notionapi.Properties{
			"title": notionapi.TitleProperty{
				Type: notionapi.PropertyTypeTitle,
				Title: []notionapi.RichText{
					{Type: "text", Text: &notionapi.Text{Content: fmt.Sprintf("Week of %s — Slate Digest", weekAgo.Format("Jan 2, 2006"))}},
				},
			},
		},
		Children: blocks,
	})
	if err != nil {
		return "", fmt.Errorf("create weekly digest: %w", err)
	}

	return string(page.ID), nil
}

// buildDashboardBlocks creates Notion blocks for the dashboard page.
func buildDashboardBlocks(ctx context.Context, store *Store, report *MetricsReport, now time.Time) []notionapi.Block {
	var blocks []notionapi.Block

	// Heading.
	blocks = append(blocks, heading1("Slate Dashboard"))
	blocks = append(blocks, paragraph(fmt.Sprintf("Generated: %s", now.Format("2006-01-02 15:04 UTC"))))

	// Status summary.
	blocks = append(blocks, heading2("Task Summary"))
	blocks = append(blocks, bulletItem(fmt.Sprintf("Open: %d", report.CurrentOpen)))
	blocks = append(blocks, bulletItem(fmt.Sprintf("Blocked: %d", report.CurrentBlocked)))

	// Priority breakdown.
	blocks = append(blocks, heading2("Tasks by Priority"))
	priorityCounts := countByPriority(ctx, store)
	for p := 0; p <= 4; p++ {
		if count, ok := priorityCounts[Priority(p)]; ok && count > 0 {
			blocks = append(blocks, bulletItem(fmt.Sprintf("P%d: %d", p, count)))
		}
	}

	// Metrics.
	blocks = append(blocks, heading2("Period Metrics"))
	blocks = append(blocks, bulletItem(fmt.Sprintf("Tasks closed: %d", report.TasksClosed)))
	blocks = append(blocks, bulletItem(fmt.Sprintf("Tasks created: %d", report.TasksCreated)))
	blocks = append(blocks, bulletItem(fmt.Sprintf("Tasks cancelled: %d", report.TasksCancelled)))
	if report.AvgCycleTime > 0 {
		blocks = append(blocks, bulletItem(fmt.Sprintf("Average cycle time: %s", report.AvgCycleTime.Round(time.Minute))))
	}

	// Blocked tasks.
	blockedTasks, _ := store.Blocked(ctx)
	if len(blockedTasks) > 0 {
		blocks = append(blocks, heading2(fmt.Sprintf("Blocked Tasks (%d)", len(blockedTasks))))
		for _, t := range blockedTasks {
			blocks = append(blocks, bulletItem(fmt.Sprintf("[%s] %s", t.ID, t.Title)))
		}
	}

	// Top assignees by throughput.
	topAssignees := countClosedByAssignee(ctx, store)
	if len(topAssignees) > 0 {
		blocks = append(blocks, heading2("Top Assignees"))
		for _, a := range topAssignees {
			blocks = append(blocks, bulletItem(fmt.Sprintf("%s: %d closed", a.name, a.count)))
		}
	}

	return blocks
}

// buildWeeklyBlocks creates Notion blocks for the weekly digest.
func buildWeeklyBlocks(ctx context.Context, store *Store, from, to time.Time) []notionapi.Block {
	var blocks []notionapi.Block

	blocks = append(blocks, heading1(fmt.Sprintf("Week of %s", from.Format("Jan 2, 2006"))))

	// Closed tasks.
	closedTasks, _ := store.List(ctx, ListParams{
		Status: statusPtr(StatusClosed),
	})
	var closedThisWeek []*Task
	for _, t := range closedTasks {
		if t.ClosedAt != nil && t.ClosedAt.After(from) && t.ClosedAt.Before(to) {
			closedThisWeek = append(closedThisWeek, t)
		}
	}
	blocks = append(blocks, heading2(fmt.Sprintf("Completed (%d)", len(closedThisWeek))))
	for _, t := range closedThisWeek {
		blocks = append(blocks, bulletItem(fmt.Sprintf("[%s] %s", t.ID, t.Title)))
	}

	// Created tasks.
	allTasks, _ := store.List(ctx, ListParams{})
	var createdThisWeek []*Task
	for _, t := range allTasks {
		if t.CreatedAt.After(from) && t.CreatedAt.Before(to) {
			createdThisWeek = append(createdThisWeek, t)
		}
	}
	blocks = append(blocks, heading2(fmt.Sprintf("Created (%d)", len(createdThisWeek))))
	for _, t := range createdThisWeek {
		blocks = append(blocks, bulletItem(fmt.Sprintf("[%s] %s", t.ID, t.Title)))
	}

	// Key decisions from checkpoints.
	blocks = append(blocks, heading2("Key Decisions"))
	decisionsFound := false
	for _, t := range closedThisWeek {
		checkpoints, _ := store.ListCheckpoints(ctx, t.ID)
		for _, cp := range checkpoints {
			if cp.Decisions != "" {
				blocks = append(blocks, bulletItem(fmt.Sprintf("[%s] %s", t.ID, cp.Decisions)))
				decisionsFound = true
			}
		}
	}
	if !decisionsFound {
		blocks = append(blocks, paragraph("No decisions recorded this week."))
	}

	// Blockers encountered.
	blockedTasks, _ := store.List(ctx, ListParams{Status: statusPtr(StatusBlocked)})
	blocks = append(blocks, heading2(fmt.Sprintf("Currently Blocked (%d)", len(blockedTasks))))
	for _, t := range blockedTasks {
		blocks = append(blocks, bulletItem(fmt.Sprintf("[%s] %s", t.ID, t.Title)))
	}
	if len(blockedTasks) == 0 {
		blocks = append(blocks, paragraph("No blocked tasks."))
	}

	// Velocity trend (last 4 weeks).
	blocks = append(blocks, heading2("Velocity Trend"))
	for i := 3; i >= 0; i-- {
		weekEnd := to.AddDate(0, 0, -7*i)
		weekStart := weekEnd.AddDate(0, 0, -7)
		throughput, _ := store.Throughput(ctx, weekStart, weekEnd)
		label := weekStart.Format("Jan 2")
		blocks = append(blocks, bulletItem(fmt.Sprintf("%s: %d tasks closed", label, throughput)))
	}

	return blocks
}

// --- Notion block helpers ---

func heading1(text string) notionapi.Block {
	return notionapi.Heading1Block{
		BasicBlock: notionapi.BasicBlock{Object: notionapi.ObjectTypeBlock, Type: notionapi.BlockTypeHeading1},
		Heading1:   notionapi.Heading{RichText: []notionapi.RichText{{Type: "text", Text: &notionapi.Text{Content: text}}}},
	}
}

func heading2(text string) notionapi.Block {
	return notionapi.Heading2Block{
		BasicBlock: notionapi.BasicBlock{Object: notionapi.ObjectTypeBlock, Type: notionapi.BlockTypeHeading2},
		Heading2:   notionapi.Heading{RichText: []notionapi.RichText{{Type: "text", Text: &notionapi.Text{Content: text}}}},
	}
}

func paragraph(text string) notionapi.Block {
	return notionapi.ParagraphBlock{
		BasicBlock: notionapi.BasicBlock{Object: notionapi.ObjectTypeBlock, Type: notionapi.BlockTypeParagraph},
		Paragraph:  notionapi.Paragraph{RichText: []notionapi.RichText{{Type: "text", Text: &notionapi.Text{Content: text}}}},
	}
}

func bulletItem(text string) notionapi.Block {
	return notionapi.BulletedListItemBlock{
		BasicBlock:       notionapi.BasicBlock{Object: notionapi.ObjectTypeBlock, Type: notionapi.BlockTypeBulletedListItem},
		BulletedListItem: notionapi.ListItem{RichText: []notionapi.RichText{{Type: "text", Text: &notionapi.Text{Content: text}}}},
	}
}

func statusPtr(s Status) *Status {
	return &s
}

// --- Dashboard query helpers ---

// countByPriority counts open/in_progress tasks by priority.
func countByPriority(ctx context.Context, store *Store) map[Priority]int {
	counts := make(map[Priority]int)
	tasks, _ := store.List(ctx, ListParams{})
	for _, t := range tasks {
		if t.Status != StatusClosed && t.Status != StatusCancelled {
			counts[t.Priority]++
		}
	}
	return counts
}

// assigneeCount is a name+count pair for top assignees.
type assigneeCount struct {
	name  string
	count int
}

// countClosedByAssignee returns assignees sorted by number of closed tasks (descending).
func countClosedByAssignee(ctx context.Context, store *Store) []assigneeCount {
	tasks, _ := store.List(ctx, ListParams{Status: statusPtr(StatusClosed)})
	counts := make(map[string]int)
	for _, t := range tasks {
		if t.Assignee != "" {
			counts[t.Assignee]++
		}
	}

	var result []assigneeCount
	for name, count := range counts {
		result = append(result, assigneeCount{name: name, count: count})
	}

	// Sort descending by count.
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].count > result[i].count {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	// Top 10.
	if len(result) > 10 {
		result = result[:10]
	}
	return result
}
