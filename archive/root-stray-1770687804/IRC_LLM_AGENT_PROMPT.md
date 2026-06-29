# IRC Client with LLM Chatbot Agent

**Task:** Design and build a simple IRC client with LLM support that uses an agent as a chatbot.

**Constraint:** Use **nothing but MCP calls** to geoffrussy for all operations.

---

## Requirements

### Application Features
- **IRC Client**: Basic IRC protocol implementation (connect to server, join channels, send/receive messages)
- **LLM Integration**: Connect to an LLM API (configurable provider like OpenAI, Anthropic, etc.)
- **Agent Mode**: LLM acts as an agent that can use geoffrussy MCP tools to interact with Geoffrussy project
- **Simple UI**: Command-line interface for chatting with an IRC channel

### Mandatory MCP Usage
**ALL** operations must use geoffrussy MCP server:

1. **Project Setup**
   - Use `geoffrussy init` (or verify project is initialized via `get_status` tool)
   - Use `create_checkpoint` to save progress before each major step
   - Use `list_checkpoints` to track progress

2. **Design Phase**
   - Use `create_checkpoint` - checkpoint: "design-start"
   - Document architecture decisions in a file (tracked by git)
   - Use `create_checkpoint` - checkpoint: "design-complete"

3. **Development Phase**
   - Use `create_checkpoint` - checkpoint: "development-start"
   - Write all code using MCP tool calls to verify project state
   - Use `get_status` and `get_stats` periodically to track progress
   - Use `create_checkpoint` - checkpoint: "development-complete"

4. **Testing Phase**
   - Use `create_checkpoint` - checkpoint: "testing-start"
   - Test IRC connection
   - Test LLM API integration
   - Test agent mode with MCP tools
   - Use `create_checkpoint` - checkpoint: "testing-complete"

### Architecture Requirements

#### IRC Client
- Connect to IRC server (configurable host/port)
- Join specified channels
- Handle IRC protocol messages (PING, PONG, JOIN, PRIVMSG, etc.)
- Maintain connection state
- Handle reconnection logic

#### LLM Integration
- Support multiple LLM providers (OpenAI, Anthropic, etc.)
- API key management (load from environment or config)
- Streaming response support (if available)
- Message history for context

#### Agent Mode
- When agent is enabled, incoming IRC messages trigger LLM calls
- LLM can use geoffrussy MCP tools to:
  - Query project status via `get_status`
  - Get statistics via `get_stats`
  - List phases via `list_phases`
  - Create checkpoints via `create_checkpoint`
- Agent responses are sent back to IRC channel

#### Configuration
- IRC server settings (host, port, nickname, channels)
- LLM provider settings (provider, model, API key)
- Agent mode toggle (on/off)
- Agent tool access permissions

---

## Implementation Steps (Using MCP Only)

### Step 1: Initialize and Plan
```python
# MCP calls required:
1. initialize() - Connect to MCP server
2. tools/call get_status - Check project state
3. tools/call create_checkpoint - checkpoint: "irc-llm-agent-start"
```

### Step 2: Design Architecture
```python
# MCP calls required:
1. tools/call create_checkpoint - checkpoint: "design-start"
2. Write architecture document
3. tools/call create_checkpoint - checkpoint: "design-complete"
```

**Architecture to document:**
```
┌─────────────────┐
│   IRC Client    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Message Handler │
└────────┬────────┘
         │
    ┌────┴────┐
    ▼         ▼
┌─────┐  ┌──────────┐
│ IRC │  │   LLM    │
│     │  │  Manager  │
└─────┘  └─────┬────┘
               │
          ┌────┴────┐
          ▼         ▼
     ┌────────┐ ┌──────────┐
     │ Agent  │ │   MCP    │
     │  Mode  │ │  Client  │
     └────────┘ └─────┬────┘
                      │
                 ┌────┴────┐
                 │Geoffrussy │
                 │   MCP   │
                 │ Server  │
                 └─────────┘
```

### Step 3: Implement IRC Client
```python
# MCP calls required:
1. tools/call create_checkpoint - checkpoint: "irc-client-start"
2. Implement IRC protocol handler
3. Test IRC connection
4. tools/call create_checkpoint - checkpoint: "irc-client-complete"
```

**Key Functions:**
- `Connect(host, port, nickname)` - Connect to IRC server
- `Join(channel)` - Join a channel
- `Send(message)` - Send message to channel
- `OnMessage(callback)` - Register message handler
- `Ping()` - Handle IRC ping/pong

### Step 4: Implement LLM Manager
```python
# MCP calls required:
1. tools/call create_checkpoint - checkpoint: "llm-manager-start"
2. Implement LLM API client
3. Test LLM integration
4. tools/call create_checkpoint - checkpoint: "llm-manager-complete"
```

**Key Functions:**
- `SetProvider(provider, model, apiKey)` - Configure LLM
- `Chat(messages, tools)` - Chat with tool support
- `ChatStream(messages, tools)` - Streaming chat

### Step 5: Implement Agent Mode
```python
# MCP calls required:
1. tools/call create_checkpoint - checkpoint: "agent-mode-start"
2. Implement tool calling with Geoffrussy MCP
3. Test agent mode
4. tools/call create_checkpoint - checkpoint: "agent-mode-complete"
```

**Key Functions:**
- `AgentChat(userMessage)` - Process message as agent
- `GetToolDefinitions()` - Fetch Geoffrussy tools via MCP
- `CallTool(toolName, arguments)` - Execute MCP tool
- `FormatToolCall(tool, result)` - Format tool response for user

### Step 6: Integrate and Test
```python
# MCP calls required:
1. tools/call create_checkpoint - checkpoint: "integration-start"
2. Integrate all components
3. End-to-end testing
4. tools/call create_checkpoint - checkpoint: "integration-complete"
```

---

## MCP Tool Definitions to Use

You must use these geoffrussy MCP tools:

### get_status
```json
{
  "name": "get_status",
  "arguments": {"projectPath": "/path/to/project"}
}
```
Returns project status, stage, progress

### get_stats
```json
{
  "name": "get_stats",
  "arguments": {"projectPath": "/path/to/project"}
}
```
Returns token usage and cost statistics

### list_phases
```json
{
  "name": "list_phases",
  "arguments": {"projectPath": "/path/to/project"}
}
```
Returns development phases and tasks

### create_checkpoint
```json
{
  "name": "create_checkpoint",
  "arguments": {
    "projectPath": "/path/to/project",
    "name": "checkpoint-name"
  }
}
```
Creates a git tag and database checkpoint

### list_checkpoints
```json
{
  "name": "list_checkpoints",
  "arguments": {"projectPath": "/path/to/project"}
}
```
Lists all checkpoints

---

## Agent Behavior Specification

When agent mode is enabled:

1. **Receive IRC message** from user
2. **Send to LLM** with:
   - System prompt: "You are an agent with access to Geoffrussy MCP tools. Use tools to help users interact with the Geoffrussy project."
   - Available tools: Fetch via `tools/list` from geoffrussy MCP
   - User message: The IRC message
3. **LLM decides** whether to call a tool
4. **Execute tool** via geoffrussy MCP `tools/call`
5. **Send result** back to LLM
6. **LLM formats** response for user
7. **Send response** to IRC channel

**Example Conversation:**

```
User in IRC: What's the project status?

Agent:
1. LLM recognizes need for project information
2. LLM calls get_status tool via MCP
3. MCP returns: "Stage: design, Progress: 25%"
4. LLM responds: "The Geoffrussy project is currently in design stage with 25% progress."
5. Agent sends to IRC: "The Geoffrussy project is currently in design stage with 25% progress."
```

---

## Configuration File

Create a `.irc-llm-agent.json` configuration file:

```json
{
  "irc": {
    "server": "irc.libera.chat",
    "port": 6667,
    "nickname": "GeoffrussyBot",
    "channels": ["#geoffrussy-dev"]
  },
  "llm": {
    "provider": "openai",
    "model": "gpt-4",
    "apiKey": "${OPENAI_API_KEY}",
    "agentMode": true,
    "toolsEnabled": true
  },
  "geoffrussy": {
    "projectPath": "/path/to/project",
    "mcpServerPath": "/path/to/geoffrussy"
  }
}
```

---

## Code Structure

```
irc-llm-agent/
├── main.go                 # Entry point
├── config/
│   └── config.go          # Configuration loading
├── irc/
│   └── client.go          # IRC protocol implementation
├── llm/
│   ├── client.go          # LLM API client
│   └── tools.go         # Tool calling support
├── agent/
│   └── agent.go          # Agent mode implementation
└── mcp/
    └── client.go          # Geoffrussy MCP client
```

---

## Success Criteria

✅ IRC client can connect and join channels
✅ LLM integration works with at least one provider (OpenAI or Anthropic)
✅ Agent mode successfully uses Geoffrussy MCP tools
✅ Agent can call get_status, get_stats, list_phases, create_checkpoint
✅ Checkpoints created at each major milestone
✅ All operations tracked via MCP tools
✅ Code is clean, documented, and follows Go best practices

---

## Deliverables

1. **Source code** - Complete IRC client with LLM and agent support
2. **Configuration example** - .irc-llm-agent.json template
3. **README.md** - Setup and usage instructions
4. **Checkpoints** - All milestones saved via MCP
5. **Receipts** - MCP protocol trace showing all tool calls
6. **Test results** - Demonstration of working IRC + LLM + Agent

---

## Getting Started

1. **Start Geoffrussy MCP server** in a separate terminal:
   ```bash
   geoffrussy mcp-server --project-path /path/to/project
   ```

2. **Initialize your build process**:
   ```python
   client = MCPClient("/path/to/geoffrussy", "/path/to/project")
   client.initialize()
   client.create_checkpoint("irc-llm-agent-start")
   ```

3. **Follow the implementation steps** using only MCP tool calls

4. **Verify each step** with `get_status` and `create_checkpoint`

5. **Final verification**:
   - Start IRC client
   - Join a test channel
   - Enable agent mode
   - Send "What's the project status?"
   - Verify agent responds with Geoffrussy MCP data

---

## Important Notes

- **NO direct file operations** - All project state via MCP tools
- **NO manual checkpoints** - Use `create_checkpoint` tool
- **Document everything** - Each checkpoint should be meaningful
- **Test incrementally** - Verify each component before moving on
- **Save all receipts** - Document every MCP call with timestamps

---

**Good luck! The IRC client will serve as a demonstration of how to build applications that use Geoffrussy MCP for agent capabilities.**
