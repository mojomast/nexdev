package controlplane

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mojomast/nexdev/internal/state"
)

// Role is the control-plane authorization level required by OpenAPI route metadata.
type Role string

const (
	RoleNone     Role = "none"
	RoleObserver Role = "observer"
	RoleOperator Role = "operator"
	RoleAdmin    Role = "admin"
	// RolePerTool is a route-level marker only; it must not be assigned to actors.
	// Use AllowsRoute for per-tool authorization checks.
	RolePerTool Role = "per-tool"
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

type authContextKey struct{}

type AuthStore interface {
	GetAuthTokenByHash(ctx context.Context, tokenHash string) (*state.AuthToken, error)
	TouchAuthTokenLastUsed(ctx context.Context, tokenID string, lastUsedAt time.Time) error
}

type AuditStore interface {
	CreateAuditRecord(ctx context.Context, record *state.AuditRecord) error
}

type Authenticator struct {
	store     AuthStore
	audits    AuditStore
	projectID string
	secret    []byte
	now       func() time.Time
	newID     func(prefix string) string
	limiter   AuthThrottle
}

type AuthenticatorConfig struct {
	Store        AuthStore
	AuditStore   AuditStore
	ProjectID    string
	ServerSecret []byte
	Now          func() time.Time
	NewID        func(prefix string) string
	Throttle     AuthThrottle
}

type AuthThrottle interface {
	Allow(key string, now time.Time) bool
}

type LocalAuthThrottle struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	attempts map[string][]time.Time
}

type AuthenticatedActor struct {
	TokenID string
	Role    Role
	Name    string
}

func NewAuthenticator(cfg AuthenticatorConfig) (*Authenticator, error) {
	if cfg.Store == nil {
		return nil, fmt.Errorf("auth store is required")
	}
	if len(cfg.ServerSecret) == 0 {
		return nil, fmt.Errorf("server secret is required")
	}
	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	newID := cfg.NewID
	if newID == nil {
		newID = randomControlPlaneID
	}
	return &Authenticator{store: cfg.Store, audits: cfg.AuditStore, projectID: cfg.ProjectID, secret: append([]byte(nil), cfg.ServerSecret...), now: now, newID: newID, limiter: cfg.Throttle}, nil
}

func NewLocalAuthThrottle(limit int, window time.Duration) *LocalAuthThrottle {
	if limit <= 0 {
		limit = 10
	}
	if window <= 0 {
		window = time.Minute
	}
	return &LocalAuthThrottle{limit: limit, window: window, attempts: map[string][]time.Time{}}
}

func (t *LocalAuthThrottle) Allow(key string, now time.Time) bool {
	if t == nil {
		return true
	}
	key = strings.TrimSpace(key)
	if key == "" {
		key = "unknown"
	}
	now = now.UTC()
	cutoff := now.Add(-t.window)
	t.mu.Lock()
	defer t.mu.Unlock()
	kept := t.attempts[key][:0]
	for _, at := range t.attempts[key] {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	if len(kept) >= t.limit {
		t.attempts[key] = kept
		return false
	}
	t.attempts[key] = append(kept, now)
	return true
}

func GenerateOpaqueToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate auth token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func HashBearerToken(serverSecret []byte, token string) string {
	mac := hmac.New(sha256.New, serverSecret)
	mac.Write([]byte(token))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (a *Authenticator) Authenticate(ctx context.Context, header string) (AuthenticatedActor, error) {
	token, ok := bearerToken(header)
	if !ok {
		return AuthenticatedActor{}, ErrUnauthorized
	}
	hash := HashBearerToken(a.secret, token)
	record, err := a.store.GetAuthTokenByHash(ctx, hash)
	if err != nil {
		return AuthenticatedActor{}, ErrUnauthorized
	}
	if !hmac.Equal([]byte(record.TokenHash), []byte(hash)) {
		return AuthenticatedActor{}, ErrUnauthorized
	}
	now := a.now().UTC()
	if record.RevokedAt != nil || (record.ExpiresAt != nil && !record.ExpiresAt.After(now)) {
		return AuthenticatedActor{}, ErrUnauthorized
	}
	role := Role(record.Role)
	if roleRank(role) == 0 {
		return AuthenticatedActor{}, ErrForbidden
	}
	if err := a.store.TouchAuthTokenLastUsed(ctx, record.ID, now); err != nil {
		return AuthenticatedActor{}, err
	}
	return AuthenticatedActor{TokenID: record.ID, Role: role, Name: record.Name}, nil
}

func ActorFromContext(ctx context.Context) (AuthenticatedActor, bool) {
	actor, ok := ctx.Value(authContextKey{}).(AuthenticatedActor)
	return actor, ok
}

func withActor(ctx context.Context, actor AuthenticatedActor) context.Context {
	return context.WithValue(ctx, authContextKey{}, actor)
}

func bearerToken(header string) (string, bool) {
	fields := strings.Fields(header)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") || fields[1] == "" {
		return "", false
	}
	return fields[1], true
}

func (a *Authenticator) Middleware(required Role, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if required == RoleNone {
			next.ServeHTTP(w, r)
			return
		}
		if a.limiter != nil && !a.limiter.Allow(authThrottleKey(r), a.now()) {
			a.auditRequest(r, AuthenticatedActor{}, "auth_throttle", "denied")
			writeError(w, r, http.StatusTooManyRequests, "rate_limited", "too many authentication attempts", nil)
			return
		}
		actor, err := a.Authenticate(r.Context(), r.Header.Get("Authorization"))
		if err != nil {
			a.auditRequest(r, AuthenticatedActor{}, "auth", "failed")
			writeError(w, r, http.StatusUnauthorized, "unauthorized", "authentication required", nil)
			return
		}
		if !Allows(actor.Role, required) {
			a.auditRequest(r, actor, "authorize", "forbidden")
			writeError(w, r, http.StatusForbidden, "forbidden", "insufficient role", nil)
			return
		}
		if roleRank(required) >= roleRank(RoleOperator) {
			a.auditRequest(r, actor, "control_request", "allowed")
		}
		next.ServeHTTP(w, r.WithContext(withActor(r.Context(), actor)))
	})
}

func authThrottleKey(r *http.Request) string {
	if r == nil {
		return "unknown"
	}
	// X-Forwarded-For is intentionally not trusted here: Nexdev's local
	// control plane does not own or verify a trusted proxy boundary, so clients
	// could spoof that header to bypass local auth throttling.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}
	return "unknown"
}

func (a *Authenticator) auditRequest(r *http.Request, actor AuthenticatedActor, action, outcome string) {
	if a.audits == nil || a.projectID == "" || r == nil {
		return
	}
	role := ""
	if actor.Role != "" {
		role = string(actor.Role)
	}
	_ = a.audits.CreateAuditRecord(r.Context(), &state.AuditRecord{
		ID:           a.newID("audit"),
		ProjectID:    a.projectID,
		RequestID:    requestID(r),
		Actor:        actor.Name,
		ActorRole:    role,
		Source:       "api",
		Action:       action,
		ResourceType: "route",
		ResourceID:   r.Method + " " + r.URL.Path,
		Outcome:      outcome,
		Details:      []byte(`{}`),
		CreatedAt:    a.now().UTC(),
	})
}

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
	case RolePerTool:
		return -1
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

func randomControlPlaneID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UTC().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}
