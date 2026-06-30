package testutil

import (
	"testing"

	"github.com/mojomast/nexdev/internal/controlplane"
)

func AuthRoles() []controlplane.Role {
	return []controlplane.Role{
		controlplane.RoleObserver,
		controlplane.RoleOperator,
		controlplane.RoleAdmin,
	}
}

func RequiredRouteRole(t testing.TB, method, path string) controlplane.Role {
	t.Helper()
	role, ok := controlplane.RouteRole(method, path)
	if !ok {
		t.Fatalf("route role not found for %s %s", method, path)
	}
	return role
}
