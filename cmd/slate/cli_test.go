//go:build e2e

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binary string

func TestMain(m *testing.M) {
	// Build the binary once for all tests.
	dir, err := os.MkdirTemp("", "slate-e2e-*")
	if err != nil {
		panic(err)
	}
	binary = filepath.Join(dir, "slate")
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		panic(string(out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func setupHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("SLATE_HOME", home)
	run(t, "config", "init", "--prefix", "st")
	return home
}

func run(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command(binary, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("slate %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
	return string(out)
}

func runExpectFail(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command(binary, args...)
	out, _ := cmd.CombinedOutput()
	return string(out)
}

func TestCLI_Version(t *testing.T) {
	out := run(t, "version")
	if !strings.Contains(out, "slate") {
		t.Errorf("version output = %q", out)
	}
}

func TestCLI_Init(t *testing.T) {
	setupHome(t)
	// Should not fail on second init.
	out := run(t, "config", "init")
	if !strings.Contains(out, "already exists") {
		t.Logf("init output: %s", out)
	}
}

func TestCLI_CreateAndShow(t *testing.T) {
	setupHome(t)

	out := run(t, "create", "Test task", "--type", "bug", "--priority", "1", "--quiet")
	id := strings.TrimSpace(out)
	if !strings.HasPrefix(id, "st-") {
		t.Fatalf("id = %q, want st-* prefix", id)
	}

	out = run(t, "show", id)
	if !strings.Contains(out, "Test task") {
		t.Errorf("show missing title, got: %s", out)
	}
}

func TestCLI_CreateJSON(t *testing.T) {
	setupHome(t)

	out := run(t, "create", "JSON test", "--json")
	var task map[string]interface{}
	if err := json.Unmarshal([]byte(out), &task); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if task["title"] != "JSON test" {
		t.Errorf("title = %v", task["title"])
	}
}

func TestCLI_List(t *testing.T) {
	setupHome(t)

	run(t, "create", "Task A")
	run(t, "create", "Task B")

	out := run(t, "list")
	if !strings.Contains(out, "Task A") || !strings.Contains(out, "Task B") {
		t.Errorf("list missing tasks: %s", out)
	}
}

func TestCLI_ListJSON(t *testing.T) {
	setupHome(t)

	run(t, "create", "J1")
	out := run(t, "list", "--json")

	var tasks []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &tasks); err != nil {
		t.Fatalf("invalid JSON list: %v\n%s", err, out)
	}
	if len(tasks) == 0 {
		t.Error("empty JSON list")
	}
}

func TestCLI_ListTree(t *testing.T) {
	setupHome(t)

	out := run(t, "create", "Parent", "--quiet")
	parentID := strings.TrimSpace(out)
	run(t, "create", "Child", "--parent", parentID)

	out = run(t, "list", "--tree")
	if !strings.Contains(out, "Parent") || !strings.Contains(out, "Child") {
		t.Errorf("tree missing tasks: %s", out)
	}
}

func TestCLI_UpdateClaim(t *testing.T) {
	setupHome(t)

	out := run(t, "create", "Claimable", "--quiet")
	id := strings.TrimSpace(out)

	out = run(t, "update", id, "--claim", "--actor", "agent-1")
	if !strings.Contains(out, "Claimed") {
		t.Errorf("claim output: %s", out)
	}
}

func TestCLI_CloseReopen(t *testing.T) {
	setupHome(t)

	out := run(t, "create", "Closeable", "--quiet")
	id := strings.TrimSpace(out)

	run(t, "close", id, "--reason", "done")
	run(t, "reopen", id)

	out = run(t, "show", id, "--json")
	var task map[string]interface{}
	json.Unmarshal([]byte(out), &task)
	if task["status"] != "open" {
		t.Errorf("status after reopen = %v, want open", task["status"])
	}
}

func TestCLI_Dependencies(t *testing.T) {
	setupHome(t)

	aOut := run(t, "create", "Task A", "--quiet")
	bOut := run(t, "create", "Task B", "--quiet")
	a := strings.TrimSpace(aOut)
	b := strings.TrimSpace(bOut)

	run(t, "dep", "add", a, b)

	out := run(t, "dep", "list", a)
	if !strings.Contains(out, b) {
		t.Errorf("dep list missing B: %s", out)
	}
}

func TestCLI_Comments(t *testing.T) {
	setupHome(t)

	out := run(t, "create", "Commented", "--quiet")
	id := strings.TrimSpace(out)

	out = run(t, "comment", "add", id, "hello world", "--quiet")
	commentID := strings.TrimSpace(out)

	out = run(t, "comment", "list", id)
	if !strings.Contains(out, "hello world") {
		t.Errorf("comment list missing: %s", out)
	}

	run(t, "comment", "edit", commentID, "updated text")
	run(t, "comment", "delete", commentID)
}

func TestCLI_Attributes(t *testing.T) {
	setupHome(t)

	out := run(t, "create", "Attr task", "--quiet")
	id := strings.TrimSpace(out)

	run(t, "attr", "define", "env", "string", "--desc", "Environment")
	run(t, "attr", "set", id, "env", "production")

	out = run(t, "attr", "get", id, "env")
	if !strings.Contains(out, "production") {
		t.Errorf("attr get: %s", out)
	}

	run(t, "attr", "delete", id, "env")
	run(t, "attr", "undefine", "env")
}

func TestCLI_Checkpoints(t *testing.T) {
	setupHome(t)

	out := run(t, "create", "CP task", "--quiet")
	id := strings.TrimSpace(out)

	run(t, "checkpoint", "add", id, "--done", "Implemented auth")
	out = run(t, "checkpoint", "list", id)
	if !strings.Contains(out, "Implemented auth") {
		t.Errorf("checkpoint list: %s", out)
	}
}

func TestCLI_ExportImport(t *testing.T) {
	setupHome(t)

	run(t, "create", "Export me")

	home := os.Getenv("SLATE_HOME")
	exportFile := filepath.Join(home, "export.jsonl")
	run(t, "export", "--file", exportFile)

	if _, err := os.Stat(exportFile); err != nil {
		t.Fatalf("export file missing: %v", err)
	}

	run(t, "import", exportFile)
}

func TestCLI_Doctor(t *testing.T) {
	setupHome(t)

	out := run(t, "doctor")
	if !strings.Contains(out, "passed") {
		t.Errorf("doctor output: %s", out)
	}
}

func TestCLI_Stats(t *testing.T) {
	setupHome(t)

	run(t, "create", "Task")
	out := run(t, "stats")
	if !strings.Contains(out, "Total tasks") {
		t.Errorf("stats output: %s", out)
	}
}

func TestCLI_StatsJSON(t *testing.T) {
	setupHome(t)

	run(t, "create", "Task")
	out := run(t, "stats", "--json")

	var stats map[string]interface{}
	if err := json.Unmarshal([]byte(out), &stats); err != nil {
		t.Fatalf("invalid JSON stats: %v\n%s", err, out)
	}
}

func TestCLI_Ready(t *testing.T) {
	setupHome(t)

	run(t, "create", "Ready task")
	out := run(t, "ready")
	if !strings.Contains(out, "Ready task") {
		t.Errorf("ready output: %s", out)
	}
}

func TestCLI_Next(t *testing.T) {
	setupHome(t)

	run(t, "create", "Pick me")
	out := run(t, "next")
	if !strings.Contains(out, "Recommended") {
		t.Errorf("next output: %s", out)
	}
}

func TestCLI_Metrics(t *testing.T) {
	setupHome(t)

	out := run(t, "create", "Metric task", "--quiet")
	id := strings.TrimSpace(out)
	run(t, "close", id, "--reason", "done")

	out = run(t, "metrics")
	if !strings.Contains(out, "Tasks created") {
		t.Errorf("metrics output: %s", out)
	}
}

func TestCLI_MetricsJSON(t *testing.T) {
	setupHome(t)

	out := run(t, "metrics", "--json")
	var report map[string]interface{}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("invalid JSON metrics: %v\n%s", err, out)
	}
}

func TestCLI_Search(t *testing.T) {
	setupHome(t)

	run(t, "create", "Authentication bug")
	run(t, "create", "Dashboard feature")

	out := run(t, "search", "auth")
	if !strings.Contains(out, "Authentication") {
		t.Errorf("search missing: %s", out)
	}
}

func TestCLI_DepCycles(t *testing.T) {
	setupHome(t)

	run(t, "create", "A")
	out := run(t, "dep", "cycles")
	if !strings.Contains(out, "No cycles") {
		t.Errorf("cycles output: %s", out)
	}
}
