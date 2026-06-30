package controlplane

import (
	"errors"
	"net"
	"strings"
	"time"
)

// Role is the control-plane authorization level required by OpenAPI route metadata.
type Role string

const (
	RoleNone     Role = "none"
	RoleObserver Role = "observer"
	RoleOperator Role = "operator"
	RoleAdmin    Role = "admin"
	RolePerTool  Role = "per-tool"
)

// RouteKey identifies OpenAPI operations without depending on generated server types.
type RouteKey struct {
	Method string
	Path   string
}

// TokenRecord mirrors the auth_tokens schema for later repository/middleware work.
type TokenRecord struct {
	ID         string
	TokenHash  string
	Role       Role
	Name       string
	CreatedAt  time.Time
	ExpiresAt  *time.Time
	RevokedAt  *time.Time
	LastUsedAt *time.Time
}

// RouteRoles is the contract-level copy of api/openapi.yaml x-nexdev-role metadata.
var RouteRoles = map[RouteKey]Role{
	{Method: "GET", Path: "/health"}:                         RoleNone,
	{Method: "GET", Path: "/status"}:                         RoleObserver,
	{Method: "GET", Path: "/plan"}:                           RoleObserver,
	{Method: "GET", Path: "/artifacts"}:                      RoleObserver,
	{Method: "GET", Path: "/events"}:                         RoleObserver,
	{Method: "GET", Path: "/runs/{run_id}/stream"}:           RoleObserver,
	{Method: "POST", Path: "/runs"}:                          RoleOperator,
	{Method: "POST", Path: "/pause"}:                         RoleOperator,
	{Method: "POST", Path: "/resume"}:                        RoleOperator,
	{Method: "POST", Path: "/skip"}:                          RoleOperator,
	{Method: "POST", Path: "/steer"}:                         RoleOperator,
	{Method: "POST", Path: "/detour"}:                        RoleOperator,
	{Method: "POST", Path: "/cancel"}:                        RoleAdmin,
	{Method: "PUT", Path: "/tasks/{task_id}"}:                RoleAdmin,
	{Method: "DELETE", Path: "/tasks/{task_id}"}:             RoleAdmin,
	{Method: "POST", Path: "/blockers/{blocker_id}/resolve"}: RoleOperator,
	{Method: "GET", Path: "/config"}:                         RoleObserver,
	{Method: "PUT", Path: "/config"}:                         RoleAdmin,
	{Method: "GET", Path: "/providers"}:                      RoleObserver,
	{Method: "POST", Path: "/providers/{name}/test"}:         RoleOperator,
	{Method: "GET", Path: "/mcp/tools"}:                      RoleObserver,
	{Method: "POST", Path: "/mcp/call"}:                      RolePerTool,
}

var ErrRemoteBindRequiresAuth = errors.New("non-loopback bind requires auth")

func RouteRole(method, path string) (Role, bool) {
	role, ok := RouteRoles[RouteKey{Method: strings.ToUpper(method), Path: path}]
	return role, ok
}

func Allows(actual Role, required Role) bool {
	if required == RoleNone {
		return true
	}
	return roleRank(actual) >= roleRank(required) && roleRank(required) > 0
}

func AllowsRoute(actual Role, required Role, toolRequired Role) bool {
	if required == RolePerTool {
		return Allows(actual, toolRequired)
	}
	return Allows(actual, required)
}

func RequireAuthForBind(bind string, authEnabled bool) error {
	if authEnabled || isLoopbackBind(bind) {
		return nil
	}
	return ErrRemoteBindRequiresAuth
}

func isLoopbackBind(bind string) bool {
	bind = strings.TrimSpace(bind)
	if bind == "" || bind == "localhost" {
		return true
	}
	if host, _, err := net.SplitHostPort(bind); err == nil {
		bind = strings.Trim(host, "[]")
	}
	ip := net.ParseIP(bind)
	return ip != nil && ip.IsLoopback()
}

func roleRank(role Role) int {
	switch role {
	case RoleObserver:
		return 1
	case RoleOperator:
		return 2
	case RoleAdmin:
		return 3
	default:
		return 0
	}
}
