# MCP Integration

Geoffrussy exposes its workflow over MCP (stdio JSON-RPC 2.0).

## Start Server

```bash
geoffrussy mcp-server --project-path /absolute/path/to/project
```

Debug mode:

```bash
geoffrussy mcp-server --project-path /absolute/path/to/project --debug
```

Server identity:

- Name: `geoffrussy`
- Transport: stdio

## Claude Desktop Example

```json
{
  "mcpServers": {
    "geoffrussy": {
      "command": "/absolute/path/to/geoffrussy",
      "args": ["mcp-server", "--project-path", "/absolute/path/to/project"]
    }
  }
}
```

## Tool Coverage

Core tools include:

- status/stats/phases/checkpoints
- interview run + answer submission
- design generation + regeneration
- devplan generation
- phase/task execution + task output + blocker handling

## Resources

Common URIs:

- `project://status`
- `project://interview`
- `project://architecture`
- `project://devplan`
- `project://phases`
- `project://checkpoints`
- `project://stats`

## Operational Notes

- Logs go to stderr, never stdout (stdout is protocol channel).
- Use absolute paths in MCP client config.
- Server and CLI share project-local state at `.geoffrussy/state.db`.

## Troubleshooting

- Missing architecture resource: run `design` stage first.
- Missing devplan resource: run `plan` stage first.
- Provider auth failures: run `geoffrussy config --set-key` and `--provider-help <provider>`.
