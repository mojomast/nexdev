package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/mojomast/nexdev/internal/state"
)

// Server represents an MCP server
type Server struct {
	info             Implementation
	capabilities     ServerCapabilities
	store            *state.Store
	toolRegistry     *ToolRegistry
	resourceRegistry *ResourceRegistry
	ctx              context.Context
	cancel           context.CancelFunc
	stdin            io.Reader
	stdout           io.Writer
	stderr           io.Writer
	debugEnabled     bool
}

// ServerConfig contains configuration for the MCP server
type ServerConfig struct {
	Name    string
	Version string
	Store   *state.Store
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	Debug   bool
}

// NewServer creates a new MCP server instance
func NewServer(config ServerConfig) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	stdin, stdout, stderr := resolveIO(config)

	server := &Server{
		info: Implementation{
			Name:    config.Name,
			Version: config.Version,
		},
		capabilities: ServerCapabilities{
			Tools: &ToolsCapability{
				ListChanged: false,
			},
			Resources: &ResourcesCapability{
				Subscribe:   false,
				ListChanged: false,
			},
		},
		store:            config.Store,
		toolRegistry:     NewToolRegistry(),
		resourceRegistry: NewResourceRegistry(),
		ctx:              ctx,
		cancel:           cancel,
		stdin:            stdin,
		stdout:           stdout,
		stderr:           stderr,
		debugEnabled:     config.Debug,
	}

	return server
}

func resolveIO(config ServerConfig) (io.Reader, io.Writer, io.Writer) {
	stdin := config.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}

	stdout := config.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}

	stderr := config.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	return stdin, stdout, stderr
}

// logDebug logs debug messages to stderr if debug mode is enabled
func (s *Server) logDebug(format string, args ...interface{}) {
	if s.debugEnabled {
		fmt.Fprintf(s.stderr, "[DEBUG] "+format+"\n", args...)
	}
}

// Start starts the MCP server and begins processing requests
func (s *Server) Start() error {
	defer s.cancel()

	scanner := bufio.NewScanner(s.stdin)
	// Increase buffer size for large messages
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // 1MB max

	for scanner.Scan() {
		select {
		case <-s.ctx.Done():
			return nil
		default:
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			if err := s.handleMessage(line); err != nil {
				s.logError("Error handling message: %v", err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading from stdin: %w", err)
	}

	return nil
}

// Stop stops the MCP server
func (s *Server) Stop() {
	s.cancel()
}

// handleMessage processes a single JSON-RPC message
func (s *Server) handleMessage(data []byte) error {
	// Try to parse as a request
	var req JSONRPCRequest
	if err := json.Unmarshal(data, &req); err != nil {
		s.logDebug("Failed to parse request: %v", err)
		return s.sendError(nil, ParseError, "Parse error", err)
	}

	s.logDebug("Received request: method=%s, id=%v", req.Method, req.ID)

	// Check JSON-RPC version
	if req.JSONRPC != "2.0" {
		return s.sendError(req.ID, InvalidRequest, "Invalid Request", fmt.Errorf("unsupported JSON-RPC version: %s", req.JSONRPC))
	}

	return s.dispatchMethod(req)
}

func (s *Server) dispatchMethod(req JSONRPCRequest) error {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized":
		s.logDebug("Received initialized notification")
		return nil
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	case "resources/list":
		return s.handleResourcesList(req)
	case "resources/read":
		return s.handleResourcesRead(req)
	case "ping":
		return s.handlePing(req)
	default:
		return s.sendError(req.ID, MethodNotFound, "Method not found", fmt.Errorf("unknown method: %s", req.Method))
	}
}

// handleInitialize handles the initialize request
func (s *Server) handleInitialize(req JSONRPCRequest) error {
	var params InitializeParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return s.sendError(req.ID, InvalidParams, "Invalid params", err)
		}
	}

	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities:    s.capabilities,
		ServerInfo:      s.info,
	}

	return s.sendResult(req.ID, result)
}

// handlePing handles ping requests
func (s *Server) handlePing(req JSONRPCRequest) error {
	return s.sendResult(req.ID, map[string]interface{}{})
}

// handleToolsList handles tools/list requests
func (s *Server) handleToolsList(req JSONRPCRequest) error {
	tools := s.toolRegistry.ListTools()
	result := ListToolsResult{
		Tools: tools,
	}
	return s.sendResult(req.ID, result)
}

// handleToolsCall handles tools/call requests
func (s *Server) handleToolsCall(req JSONRPCRequest) error {
	var params CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.sendError(req.ID, InvalidParams, "Invalid params", err)
	}

	s.logDebug("Tool call: name=%s, args=%+v", params.Name, params.Arguments)

	s.logDebug("Executing tool: %s", params.Name)
	result, err := s.toolRegistry.CallTool(s.ctx, params.Name, params.Arguments)
	if err != nil {
		s.logDebug("Tool execution error: %v", err)
		return s.sendError(req.ID, InternalError, "Tool execution failed", err)
	}

	// Check if tool returned an error via isError flag
	if result.IsError {
		s.logDebug("Tool returned error result")
		var errMsg string
		if len(result.Content) > 0 {
			errMsg = result.Content[0].Text
		}
		if errMsg == "" {
			errMsg = "Tool execution failed"
		}
		return s.sendError(req.ID, InternalError, "Tool execution failed", fmt.Errorf("%s", errMsg))
	}

	s.logDebug("Tool execution successful")
	return s.sendResult(req.ID, result)
}

// handleResourcesList handles resources/list requests
func (s *Server) handleResourcesList(req JSONRPCRequest) error {
	resources := s.resourceRegistry.ListResources()
	result := ListResourcesResult{
		Resources: resources,
	}
	return s.sendResult(req.ID, result)
}

// handleResourcesRead handles resources/read requests
func (s *Server) handleResourcesRead(req JSONRPCRequest) error {
	var params ReadResourceParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.sendError(req.ID, InvalidParams, "Invalid params", err)
	}

	result, err := s.resourceRegistry.ReadResource(s.ctx, params.URI)
	if err != nil {
		return s.sendError(req.ID, InternalError, "Resource read failed", err)
	}

	return s.sendResult(req.ID, result)
}

// sendResult sends a successful JSON-RPC response
func (s *Server) sendResult(id interface{}, result interface{}) error {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	s.logDebug("Sending success response: id=%v", id)
	return s.sendResponse(response)
}

// sendError sends an error JSON-RPC response
func (s *Server) sendError(id interface{}, code int, message string, err error) error {
	rpcErr := &RPCError{
		Code:    code,
		Message: message,
	}
	if err != nil {
		rpcErr.Data = err.Error()
	}

	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   rpcErr,
	}
	s.logDebug("Sending error response: id=%v, code=%d, message=%s", id, code, message)
	return s.sendResponse(response)
}

// sendResponse sends a JSON-RPC response to stdout
func (s *Server) sendResponse(response JSONRPCResponse) error {
	data, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	// Write response followed by newline
	if _, err := s.stdout.Write(data); err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}
	if _, err := s.stdout.Write([]byte("\n")); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	return nil
}

// logError logs an error message to stderr
func (s *Server) logError(format string, args ...interface{}) {
	fmt.Fprintf(s.stderr, format+"\n", args...)
}

// GetToolRegistry returns the tool registry for registering tools
func (s *Server) GetToolRegistry() *ToolRegistry {
	return s.toolRegistry
}

// GetResourceRegistry returns the resource registry for registering resources
func (s *Server) GetResourceRegistry() *ResourceRegistry {
	return s.resourceRegistry
}
