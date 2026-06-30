package controlplane

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRoleHierarchy(t *testing.T) {
	tests := []struct {
		name     string
		actual   Role
		required Role
		want     bool
	}{
		{name: "none allows unauthenticated", actual: "", required: RoleNone, want: true},
		{name: "observer reads observer", actual: RoleObserver, required: RoleObserver, want: true},
		{name: "operator includes observer", actual: RoleOperator, required: RoleObserver, want: true},
		{name: "admin includes operator", actual: RoleAdmin, required: RoleOperator, want: true},
		{name: "observer cannot operate", actual: RoleObserver, required: RoleOperator, want: false},
		{name: "operator cannot admin", actual: RoleOperator, required: RoleAdmin, want: false},
		{name: "unknown role denied", actual: Role("root"), required: RoleObserver, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Allows(tt.actual, tt.required); got != tt.want {
				t.Fatalf("Allows(%q, %q) = %v, want %v", tt.actual, tt.required, got, tt.want)
			}
		})
	}
}

func TestRouteRoleCoverage(t *testing.T) {
	want := map[RouteKey]Role{
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

	if len(RouteRoles) != len(want) {
		t.Fatalf("RouteRoles has %d entries, want %d", len(RouteRoles), len(want))
	}
	for route, wantRole := range want {
		gotRole, ok := RouteRole(route.Method, route.Path)
		if !ok {
			t.Fatalf("missing route role for %s %s", route.Method, route.Path)
		}
		if gotRole != wantRole {
			t.Fatalf("%s %s role = %q, want %q", route.Method, route.Path, gotRole, wantRole)
		}
	}
	if gotRole, ok := RouteRole("post", "/runs"); !ok || gotRole != RoleOperator {
		t.Fatalf("RouteRole should normalize method case, got %q ok=%v", gotRole, ok)
	}
}

func TestRouteRolesMatchOpenAPIMetadata(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	var doc struct {
		Paths map[string]map[string]struct {
			Role string `yaml:"x-nexdev-role"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("openapi yaml must parse: %v", err)
	}

	openAPIRoles := make(map[RouteKey]Role)
	for path, pathItem := range doc.Paths {
		for method, operation := range pathItem {
			switch strings.ToUpper(method) {
			case "GET", "POST", "PUT", "DELETE", "PATCH":
				openAPIRoles[RouteKey{Method: strings.ToUpper(method), Path: path}] = Role(operation.Role)
			}
		}
	}

	if len(RouteRoles) != len(openAPIRoles) {
		t.Fatalf("RouteRoles has %d entries, OpenAPI has %d role metadata entries", len(RouteRoles), len(openAPIRoles))
	}
	for route, wantRole := range openAPIRoles {
		if gotRole, ok := RouteRoles[route]; !ok || gotRole != wantRole {
			t.Fatalf("RouteRoles[%s %s] = %q ok=%v, want %q", route.Method, route.Path, gotRole, ok, wantRole)
		}
	}
}

func TestMutatingRouteRoleExpectations(t *testing.T) {
	operatorRoutes := []RouteKey{
		{Method: "POST", Path: "/runs"},
		{Method: "POST", Path: "/pause"},
		{Method: "POST", Path: "/resume"},
		{Method: "POST", Path: "/skip"},
		{Method: "POST", Path: "/steer"},
		{Method: "POST", Path: "/detour"},
		{Method: "POST", Path: "/blockers/{blocker_id}/resolve"},
		{Method: "POST", Path: "/providers/{name}/test"},
	}
	for _, route := range operatorRoutes {
		if got := RouteRoles[route]; got != RoleOperator {
			t.Fatalf("%s %s role = %q, want operator", route.Method, route.Path, got)
		}
	}

	adminRoutes := []RouteKey{
		{Method: "POST", Path: "/cancel"},
		{Method: "PUT", Path: "/tasks/{task_id}"},
		{Method: "DELETE", Path: "/tasks/{task_id}"},
		{Method: "PUT", Path: "/config"},
	}
	for _, route := range adminRoutes {
		if got := RouteRoles[route]; got != RoleAdmin {
			t.Fatalf("%s %s role = %q, want admin", route.Method, route.Path, got)
		}
	}
}

func TestPerToolRouteDelegatesToToolRole(t *testing.T) {
	if !AllowsRoute(RoleOperator, RolePerTool, RoleOperator) {
		t.Fatal("operator should be allowed for an operator MCP tool")
	}
	if AllowsRoute(RoleObserver, RolePerTool, RoleOperator) {
		t.Fatal("observer should not be allowed for an operator MCP tool")
	}
	if !AllowsRoute(RoleAdmin, RolePerTool, RoleAdmin) {
		t.Fatal("admin should be allowed for an admin MCP tool")
	}
}

func TestRequireAuthForBind(t *testing.T) {
	tests := []struct {
		name        string
		bind        string
		authEnabled bool
		wantErr     bool
	}{
		{name: "loopback ipv4 without auth", bind: "127.0.0.1", wantErr: false},
		{name: "loopback hostport without auth", bind: "127.0.0.1:7432", wantErr: false},
		{name: "loopback ipv6 without auth", bind: "[::1]:7432", wantErr: false},
		{name: "localhost without auth", bind: "localhost", wantErr: false},
		{name: "remote without auth", bind: "0.0.0.0", wantErr: true},
		{name: "remote hostport without auth", bind: "192.168.1.10:7432", wantErr: true},
		{name: "remote with auth", bind: "0.0.0.0", authEnabled: true, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RequireAuthForBind(tt.bind, tt.authEnabled)
			if tt.wantErr {
				if !errors.Is(err, ErrRemoteBindRequiresAuth) {
					t.Fatalf("RequireAuthForBind() error = %v, want ErrRemoteBindRequiresAuth", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("RequireAuthForBind() error = %v, want nil", err)
			}
		})
	}
}
