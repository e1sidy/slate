package slate

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RunHooks executes shell hooks from config for the given event.
func RunHooks(cfg *Config, event Event) {
	if cfg == nil {
		return
	}

	var hooks []HookDef
	switch event.Type {
	case EventStatusChanged:
		hooks = cfg.Hooks.OnStatusChange
	case EventCreated:
		hooks = cfg.Hooks.OnCreate
	case EventCommented:
		hooks = cfg.Hooks.OnComment
	case EventClosed:
		hooks = cfg.Hooks.OnClose
	case EventAssigned:
		hooks = cfg.Hooks.OnAssign
	default:
		return
	}

	for _, h := range hooks {
		if hookMatchesFilter(h, event) {
			if h.Webhook != "" {
				go executeWebhook(h, event)
			} else if h.Command != "" {
				go executeHook(h.Command, event)
			}
		}
	}
}

// hookMatchesFilter checks if a hook's filter matches the event.
func hookMatchesFilter(h HookDef, event Event) bool {
	if len(h.Filter) == 0 {
		return true
	}
	for key, val := range h.Filter {
		switch key {
		case "new_status":
			if event.NewValue != val {
				return false
			}
		case "old_status":
			if event.OldValue != val {
				return false
			}
		case "assignee":
			if event.Actor != val {
				return false
			}
		}
	}
	return true
}

// expandHookVars replaces template variables in a hook command.
func expandHookVars(command string, event Event) string {
	r := strings.NewReplacer(
		"{id}", event.TaskID,
		"{old}", event.OldValue,
		"{new}", event.NewValue,
		"{actor}", event.Actor,
		"{field}", event.Field,
	)
	return r.Replace(command)
}

// executeHook runs a shell command, logging to hooks.log on failure.
func executeHook(command string, event Event) {
	expanded := expandHookVars(command, event)
	cmd := exec.Command("sh", "-c", expanded)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logHookError(expanded, err, output)
	}
}

func logHookError(command string, err error, output []byte) {
	logPath := filepath.Join(DefaultSlateHome(), "hooks.log")
	f, ferr := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if ferr != nil {
		fmt.Fprintf(os.Stderr, "hook error: %s: %v\n", command, err)
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "[%s] hook error: %s: %v\n", timeNowUTC().Format(timeFormat), command, err)
	if len(output) > 0 {
		fmt.Fprintf(f, "  output: %s\n", string(output))
	}
}

// executeWebhook sends an HTTP request for a webhook hook.
func executeWebhook(h HookDef, event Event) {
	method := h.Method
	if method == "" {
		method = "POST"
	}

	body := h.Body
	if body == "" {
		body = fmt.Sprintf(`{"task_id":"%s","event":"%s","old":"%s","new":"%s","actor":"%s"}`,
			event.TaskID, event.Type, event.OldValue, event.NewValue, event.Actor)
	} else {
		body = expandHookVars(body, event)
	}

	url := expandHookVars(h.Webhook, event)

	req, err := http.NewRequest(method, url, bytes.NewBufferString(body))
	if err != nil {
		logHookError("webhook: "+url, err, nil)
		return
	}

	// Default headers.
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range h.Headers {
		req.Header.Set(k, expandHookVars(v, event))
	}

	timeout := h.Timeout
	if timeout <= 0 {
		timeout = 10
	}
	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		logHookError("webhook: "+url, err, nil)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		logHookError("webhook: "+url,
			fmt.Errorf("HTTP %d", resp.StatusCode), nil)
	}
}

// EnableHooks registers a catch-all listener that fires config hooks.
func EnableHooks(store *Store, cfg *Config) {
	allEvents := []EventType{
		EventCreated, EventUpdated, EventStatusChanged,
		EventCommented, EventAssigned, EventClosed,
		EventDependencyAdded, EventDependencyRemoved,
	}
	for _, et := range allEvents {
		eventType := et // capture
		store.On(eventType, func(e Event) {
			RunHooks(cfg, e)
		})
	}
}
