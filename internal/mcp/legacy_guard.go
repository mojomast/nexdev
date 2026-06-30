package mcp

// LegacyStdioRegistrationEnabled defaults false because the imported stdio MCP
// handlers bypass Nexdev M11 control-plane auth and service boundaries.
var LegacyStdioRegistrationEnabled bool
