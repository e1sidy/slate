package slate

import (
	"context"
	"fmt"
	"sort"
)

// CriticalPathResult contains the analysis of the dependency DAG.
type CriticalPathResult struct {
	Path          []*Task `json:"critical_path"`
	TotalEstimate int     `json:"total_estimate_hours"`
	Bottlenecks   []*Task `json:"bottlenecks"`
	Parallel      []*Task `json:"parallel"`
}

// CriticalPath analyzes the dependency DAG to find the critical path,
// bottleneck tasks, and parallelizable work.
func (s *Store) CriticalPath(ctx context.Context) (*CriticalPathResult, error) {
	// Get all open/in_progress tasks.
	tasks, err := s.List(ctx, ListParams{
		ExcludeStatuses: []Status{StatusClosed, StatusCancelled},
	})
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	if len(tasks) == 0 {
		return &CriticalPathResult{}, nil
	}

	// Compute median estimate per type for fallback.
	medians := computeMedianEstimates(tasks)

	// Build adjacency list (from → to for blocking deps).
	taskMap := make(map[string]*Task)
	for _, t := range tasks {
		taskMap[t.ID] = t
	}

	// Graph: edges[A] = [B, C] means A blocks B and C.
	edges := make(map[string][]string)
	inDeg := make(map[string]int)
	for _, t := range tasks {
		inDeg[t.ID] = 0
	}

	for _, t := range tasks {
		deps, _ := s.ListDependents(ctx, t.ID)
		for _, d := range deps {
			if d.Type != Blocks && d.Type != ConditionalBlocks {
				continue
			}
			if _, ok := taskMap[d.FromID]; !ok {
				continue // blocker not in active set
			}
			edges[d.FromID] = append(edges[d.FromID], t.ID)
			inDeg[t.ID]++
		}
	}

	// Topological sort + longest path computation.
	// dist[v] = longest path ending at v (in hours).
	dist := make(map[string]int)
	prev := make(map[string]string) // for path reconstruction

	// Kahn's algorithm for topological order.
	var queue []string
	for _, t := range tasks {
		if inDeg[t.ID] == 0 {
			queue = append(queue, t.ID)
			dist[t.ID] = taskEstimate(taskMap[t.ID], medians)
		}
	}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		for _, next := range edges[curr] {
			estNext := taskEstimate(taskMap[next], medians)
			newDist := dist[curr] + estNext
			if newDist > dist[next] {
				dist[next] = newDist
				prev[next] = curr
			}
			inDeg[next]--
			if inDeg[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	// Find the task with longest distance = end of critical path.
	var endID string
	maxDist := 0
	for id, d := range dist {
		if d > maxDist {
			maxDist = d
			endID = id
		}
	}

	// Reconstruct critical path.
	var path []*Task
	for id := endID; id != ""; id = prev[id] {
		if t, ok := taskMap[id]; ok {
			path = append([]*Task{t}, path...)
		}
	}

	// Bottlenecks: tasks with most downstream dependents.
	type scored struct {
		task  *Task
		score int
	}
	var bottleneckCandidates []scored
	for _, t := range tasks {
		count := countDownstream(t.ID, edges)
		if count > 0 {
			bottleneckCandidates = append(bottleneckCandidates, scored{t, count})
		}
	}
	sort.Slice(bottleneckCandidates, func(i, j int) bool {
		return bottleneckCandidates[i].score > bottleneckCandidates[j].score
	})
	var bottlenecks []*Task
	for i, bc := range bottleneckCandidates {
		if i >= 5 {
			break
		}
		bottlenecks = append(bottlenecks, bc.task)
	}

	// Parallel: open tasks with no mutual dependency.
	var parallel []*Task
	for _, t := range tasks {
		if t.Status == StatusOpen && inDeg[t.ID] == 0 && len(edges[t.ID]) == 0 {
			parallel = append(parallel, t)
		}
	}

	return &CriticalPathResult{
		Path:          path,
		TotalEstimate: maxDist,
		Bottlenecks:   bottlenecks,
		Parallel:      parallel,
	}, nil
}

// taskEstimate returns a task's estimate, falling back to the type median.
func taskEstimate(t *Task, medians map[TaskType]int) int {
	if t.Estimate > 0 {
		return t.Estimate
	}
	if med, ok := medians[t.Type]; ok && med > 0 {
		return med
	}
	return 1 // default 1h
}

// computeMedianEstimates returns the median estimate for each task type.
func computeMedianEstimates(tasks []*Task) map[TaskType]int {
	byType := make(map[TaskType][]int)
	for _, t := range tasks {
		if t.Estimate > 0 {
			byType[t.Type] = append(byType[t.Type], t.Estimate)
		}
	}
	medians := make(map[TaskType]int)
	for tp, estimates := range byType {
		sort.Ints(estimates)
		mid := len(estimates) / 2
		medians[tp] = estimates[mid]
	}
	return medians
}

// countDownstream counts transitive downstream tasks via BFS.
func countDownstream(id string, edges map[string][]string) int {
	visited := make(map[string]bool)
	queue := []string{id}
	visited[id] = true
	count := 0
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		for _, next := range edges[curr] {
			if !visited[next] {
				visited[next] = true
				queue = append(queue, next)
				count++
			}
		}
	}
	return count
}
