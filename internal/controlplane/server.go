package controlplane

import (
	"context"
	"net/http"
	"time"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/detour"
	"github.com/mojomast/nexdev/internal/executor"
	"github.com/mojomast/nexdev/internal/state"
)

type ServerConfig struct {
	Bind                 string
	AuthRequired         bool
	ServerSecret         []byte
	ProjectID            string
	CORSAllowedOrigins   []string
	HeartbeatInterval    time.Duration
	ClientQueueMaxEvents int
	ReplayMaxEvents      int
	RetryMS              int
}

type RunStarter interface {
	StartRun(ctx context.Context, req StartRunRequest) (*state.Run, error)
}

type ProviderTester interface {
	TestProvider(ctx context.Context, name string) (map[string]any, error)
}

type DetourRequester interface {
	Request(ctx context.Context, req contract.DetourRequest) (contract.DetourResult, error)
	RequestForBlocker(ctx context.Context, runID, blockerID string) (contract.DetourResult, error)
}

type Server struct {
	cfg           ServerConfig
	store         *state.Store
	authenticator *Authenticator
	publisher     *Publisher
	executor      executor.Control
	detours       DetourRequester
	runStarter    RunStarter
	providerTests ProviderTester
	mux           *http.ServeMux
}

func NewServer(cfg ServerConfig, store *state.Store, opts ...Option) (*Server, error) {
	if cfg.Bind == "" {
		cfg.Bind = "127.0.0.1"
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = 15 * time.Second
	}
	if cfg.ClientQueueMaxEvents <= 0 {
		cfg.ClientQueueMaxEvents = 1000
	}
	if cfg.ReplayMaxEvents <= 0 {
		cfg.ReplayMaxEvents = 10000
	}
	if cfg.RetryMS <= 0 {
		cfg.RetryMS = 3000
	}
	if err := RequireAuthForBind(cfg.Bind, cfg.AuthRequired); err != nil {
		return nil, err
	}
	server := &Server{cfg: cfg, store: store, publisher: NewPublisher(store, cfg.ClientQueueMaxEvents), mux: http.NewServeMux()}
	for _, opt := range opts {
		opt(server)
	}
	if cfg.AuthRequired {
		auth, err := NewAuthenticator(AuthenticatorConfig{Store: store, AuditStore: store, ProjectID: cfg.ProjectID, ServerSecret: cfg.ServerSecret})
		if err != nil {
			return nil, err
		}
		server.authenticator = auth
	}
	server.registerRoutes()
	return server, nil
}

type Option func(*Server)

func WithExecutor(control executor.Control) Option {
	return func(s *Server) { s.executor = control }
}

func WithDetourManager(manager *detour.WorkflowManager) Option {
	return func(s *Server) { s.detours = manager }
}

func WithDetourRequester(requester DetourRequester) Option {
	return func(s *Server) { s.detours = requester }
}

func WithRunStarter(starter RunStarter) Option {
	return func(s *Server) { s.runStarter = starter }
}

func WithProviderTester(tester ProviderTester) Option {
	return func(s *Server) { s.providerTests = tester }
}

func (s *Server) Handler() http.Handler {
	return s.withCORS(s.mux)
}

func (s *Server) Publisher() *Publisher {
	return s.publisher
}
