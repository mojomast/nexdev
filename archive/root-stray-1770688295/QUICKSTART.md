# geoffrussy Quick Start

## 1) Build

```bash
git clone https://github.com/mojomast/geoffrussy.git
cd geoffrussy
make build
./bin/geoffrussy version
```

## 2) Initialize in your project

```bash
cd /path/to/your/project
geoffrussy init
```

This creates project-local state at `.geoffrussy/state.db`.

## 3) Configure providers and models

```bash
geoffrussy config --set-key
geoffrussy config --set-model
```

Optional helper:

```bash
geoffrussy config --provider-help openrouter
```

## 4) Run the pipeline

```bash
geoffrussy interview
geoffrussy design
geoffrussy plan
geoffrussy review
geoffrussy develop
```

## 5) Monitor and control

```bash
geoffrussy status
geoffrussy stats
geoffrussy quota --refresh
```

## 6) Recovery and iteration

```bash
geoffrussy checkpoint --name before-major-change
geoffrussy rollback
geoffrussy resume
geoffrussy navigate --list
```

## Optional: MCP

```bash
geoffrussy mcp-server --project-path /absolute/path/to/project --debug
```

See `docs/mcp-integration.md`.
