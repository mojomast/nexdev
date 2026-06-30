package contract

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

var requiredOpenAPIRoutes = map[string]string{
	"GET /health":                         "none",
	"GET /status":                         "observer",
	"GET /plan":                           "observer",
	"GET /artifacts":                      "observer",
	"GET /events":                         "observer",
	"GET /runs/{run_id}/stream":           "observer",
	"POST /runs":                          "operator",
	"POST /pause":                         "operator",
	"POST /resume":                        "operator",
	"POST /skip":                          "operator",
	"POST /steer":                         "operator",
	"POST /detour":                        "operator",
	"POST /cancel":                        "admin",
	"PUT /tasks/{task_id}":                "admin",
	"DELETE /tasks/{task_id}":             "admin",
	"POST /blockers/{blocker_id}/resolve": "operator",
	"GET /config":                         "observer",
	"PUT /config":                         "admin",
	"GET /providers":                      "observer",
	"POST /providers/{name}/test":         "operator",
	"GET /mcp/tools":                      "observer",
	"POST /mcp/call":                      "per-tool",
}

func TestOpenAPIRouteCoverageAndRoles(t *testing.T) {
	var doc map[string]any
	data, err := os.ReadFile(filepath.Join("..", "..", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("openapi yaml must parse: %v", err)
	}

	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatal("openapi paths object missing")
	}

	for route, wantRole := range requiredOpenAPIRoutes {
		method, path := splitRoute(t, route)
		pathItem, ok := paths[path].(map[string]any)
		if !ok {
			t.Fatalf("missing path %s", path)
		}
		operation, ok := pathItem[method].(map[string]any)
		if !ok {
			t.Fatalf("missing operation %s", route)
		}
		if gotRole, _ := operation["x-nexdev-role"].(string); gotRole != wantRole {
			t.Fatalf("%s role = %q, want %q", route, gotRole, wantRole)
		}
	}
}

func TestOpenAPICommonSchemasExist(t *testing.T) {
	var doc map[string]any
	data, err := os.ReadFile(filepath.Join("..", "..", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("openapi yaml must parse: %v", err)
	}

	components := doc["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)
	for _, name := range []string{
		"ErrorResponse",
		"StatusSnapshot",
		"RunSnapshot",
		"Plan",
		"ArtifactManifest",
		"ArtifactItem",
		"EventEnvelope",
		"ProviderStatus",
		"RedactedConfig",
		"TaskMutation",
		"StartRunRequest",
		"ControlRequest",
		"DetourRequest",
		"DetourResult",
		"BlockerResolveRequest",
		"MCPTool",
		"MCPCallRequest",
	} {
		if _, ok := schemas[name]; !ok {
			t.Fatalf("missing schema %s", name)
		}
	}
}

func splitRoute(t *testing.T, route string) (method string, path string) {
	t.Helper()
	for i := range route {
		if route[i] == ' ' {
			return lowerASCII(route[:i]), route[i+1:]
		}
	}
	t.Fatalf("invalid route %q", route)
	return "", ""
}

func lowerASCII(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}
