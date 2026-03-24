# Webhook Hooks

Send HTTP requests on task events instead of (or alongside) shell commands.

## Configuration

```yaml
# slate.yaml
hooks:
  on_status_change:
    - webhook: "https://hooks.slack.com/services/..."
      method: POST
      headers:
        Content-Type: "application/json"
      body: '{"text": "Task {id} changed to {new} by {actor}"}'
      filter:
        new_status: "closed"
  on_create:
    - webhook: "https://api.example.com/tasks"
      body: '{"task_id": "{id}", "event": "created"}'
      timeout: 5
```

## Fields

| Field | Default | Description |
|-------|---------|-------------|
| `webhook` | — | URL to send request to |
| `method` | POST | HTTP method |
| `headers` | Content-Type: application/json | HTTP headers |
| `body` | auto-generated JSON | Request body with template variables |
| `timeout` | 10 | Timeout in seconds |
| `filter` | — | Same filter syntax as shell hooks |

## Template Variables

`{id}`, `{old}`, `{new}`, `{actor}`, `{field}` — same as shell hooks.

## Behavior

- Webhooks fire asynchronously (don't block the mutation)
- Errors logged to `~/.slate/hooks.log`
- HTTP 4xx/5xx responses logged as errors
