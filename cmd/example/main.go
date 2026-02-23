// Command example demonstrates a minimal Broth application.
//
// Run:
//
//	go run ./cmd/example
//
// Then test:
//
//	curl http://localhost:8080/health
//	curl http://localhost:8080/greeting/
//	curl http://localhost:8080/greeting/World
//	curl -X POST -H "Content-Type: application/json" -d '{"name":"Gopher"}' http://localhost:8080/greeting/
//	curl http://localhost:8080/account/login
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"time"

	"github.com/source-maker/broth"
	"github.com/source-maker/broth/auth"
	"github.com/source-maker/broth/config"
	"github.com/source-maker/broth/db"
	"github.com/source-maker/broth/job"
	brothlog "github.com/source-maker/broth/log"
	"github.com/source-maker/broth/middleware"
	"github.com/source-maker/broth/modules/account"
	"github.com/source-maker/broth/render"
	"github.com/source-maker/broth/router"
	"github.com/source-maker/broth/schedule"
	"github.com/source-maker/broth/session"
)

// serverConfig holds the server configuration bound from environment variables.
type serverConfig struct {
	Addr         string `env:"APP_ADDR" default:":8080"`
	Env          string `env:"APP_ENV" default:"development"`
	LogLevel     string `env:"LOG_LEVEL" default:"info"`
	DatabaseURL  string `env:"DATABASE_URL"`
	SessionKey   string `env:"SESSION_KEY,required"`
	JWTSecret    string `env:"JWT_SECRET,required"`
	CORSOrigins  string `env:"CORS_ORIGINS" default:"*"`
}

func main() {
	// 1. Load configuration from environment variables.
	var cfg serverConfig
	config.MustBind(&cfg)

	// 2. Create the logger.
	level := parseLogLevel(cfg.LogLevel)
	logger := brothlog.New(level)
	logger.Info(nil, "starting broth example",
		"env", cfg.Env,
		"addr", cfg.Addr,
		"go_version", runtime.Version(),
	)

	// 3. Set up session store.
	sessionKey := []byte(cfg.SessionKey)
	if len(sessionKey) < 32 {
		sessionKey = append(sessionKey, make([]byte, 32-len(sessionKey))...)
	}
	cookieStore, err := session.NewCookieStore(sessionKey[:32])
	if err != nil {
		slog.Error("session store init failed", "error", err)
		os.Exit(1)
	}

	// 4. Set up auth token service.
	tokenSvc := auth.NewTokenService([]byte(cfg.JWTSecret), 24*time.Hour)

	// 5. Set up renderer.
	renderer := render.New(render.Config{
		TemplateDir:  "templates",
		LayoutDir:    "layouts",
		ComponentDir: "components",
		Reload:       cfg.Env == "development",
	}, os.DirFS("templates"), logger)

	// 6. Create the application and apply global middleware.
	app := broth.New(logger)
	app.Use(
		middleware.RequestID,
		middleware.Recovery(logger.Slog()),
		middleware.Logger(logger.Slog()),
		middleware.ContextDetect(),
		middleware.SecurityHeaders(),
		middleware.Session(cookieStore, middleware.SessionConfig{
			CookieName: "broth_session",
			Path:       "/",
			Secure:     cfg.Env == "production",
			MaxAge:     86400,
		}),
		middleware.CSRF(),
		middleware.CORS(middleware.CORSConfig{
			AllowedOrigins: []string{cfg.CORSOrigins},
		}),
		middleware.RateLimit(middleware.RateLimitConfig{RPS: 100, Burst: 50}),
	)

	// 7. Register a health check endpoint on the root router.
	app.Router().Handle(router.Route{
		Pattern: "GET /health",
		Handler: http.HandlerFunc(healthHandler),
	})

	// 8. Set up database (optional) and job system.
	var (
		database *db.Database
		enqueuer *job.Enqueuer
		leader   schedule.LeaderElector
	)

	if cfg.DatabaseURL != "" {
		// PostgreSQL mode: use persistent job backend + leader election.
		var err error
		database, err = db.Open(db.DefaultConfig(cfg.DatabaseURL), logger)
		if err != nil {
			slog.Error("database open failed", "error", err)
			os.Exit(1)
		}
		logger.Info(nil, "database connected", "url", cfg.DatabaseURL)

		backend := job.NewPgBackend(database.DB())
		enqueuer = job.NewEnqueuer(logger, job.WithBackend(backend), job.WithQueueSize(100))
		leader = schedule.NewPgLeaderElector(database.DB())
	} else {
		// In-memory mode: no database required.
		logger.Info(nil, "no DATABASE_URL set, using in-memory job queue")
		enqueuer = job.NewEnqueuer(logger, job.WithQueueSize(100))
	}

	worker := job.NewWorker(enqueuer, job.WorkerConfig{
		Concurrency:     4,
		ShutdownTimeout: 10 * time.Second,
	}, logger)

	// 9. Set up scheduler with sample cron jobs.
	scheduler := schedule.NewScheduler(enqueuer, logger, leader,
		schedule.WithOnError(func(defName string, err error) {
			logger.Error(nil, "schedule: error callback", "job", defName, "error", err)
		}),
	)
	scheduler.Register(
		schedule.Definition{
			Name:    "example-heartbeat",
			Cron:    "@every 30s",
			Job:     &heartbeatJob{logger: logger},
			Overlap: false,
		},
	)

	// 10. Register feature modules.
	repo := newInMemoryRepo()
	hasher := auth.NewBcryptHasher(0)
	accountMod := account.NewModule(repo, hasher, tokenSvc, renderer, logger)

	greetingMod := newGreetingModule(logger)
	app.Register(accountMod, greetingMod)
	app.Mount()

	// 11. Set up graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := app.Start(ctx); err != nil {
		slog.Error("app start failed", "error", err)
		os.Exit(1)
	}

	// 12. Start background workers.
	worker.Start(ctx)
	go scheduler.Start(ctx)

	// 13. Start the HTTP server.
	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      app.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info(nil, "server listening", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// 14. Wait for interrupt signal, then shut down gracefully.
	<-ctx.Done()
	logger.Info(nil, "shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := scheduler.Shutdown(shutdownCtx); err != nil {
		logger.Warn(nil, "scheduler shutdown error", "error", err)
	}
	if err := worker.Shutdown(shutdownCtx); err != nil {
		logger.Warn(nil, "worker shutdown error", "error", err)
	}
	if err := app.Shutdown(shutdownCtx); err != nil {
		logger.Warn(nil, "app shutdown error", "error", err)
	}
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Warn(nil, "server shutdown error", "error", err)
	}
	if pgLeader, ok := leader.(*schedule.PgLeaderElector); ok {
		if err := pgLeader.Close(shutdownCtx); err != nil {
			logger.Warn(nil, "leader elector close error", "error", err)
		}
	}
	if database != nil {
		if err := database.Close(); err != nil {
			logger.Warn(nil, "database close error", "error", err)
		}
	}
	logger.Info(nil, "shutdown complete")
}

// ---------------------------------------------------------------------------
// Sample background job
// ---------------------------------------------------------------------------

// heartbeatJob is a sample scheduled job that logs a heartbeat.
type heartbeatJob struct {
	logger *brothlog.Logger
}

func (j *heartbeatJob) JobName() string { return "heartbeat" }
func (j *heartbeatJob) Execute(ctx context.Context) error {
	j.logger.Info(ctx, "heartbeat: alive", "time", time.Now().UTC().Format(time.RFC3339))
	return nil
}

// ---------------------------------------------------------------------------
// In-memory account repository (for demonstration only)
// ---------------------------------------------------------------------------

type inMemoryRepo struct {
	mu    sync.RWMutex
	users []*account.User
	nextID int64
}

func newInMemoryRepo() *inMemoryRepo {
	return &inMemoryRepo{nextID: 1}
}

func (r *inMemoryRepo) Create(_ context.Context, user *account.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	user.ID = r.nextID
	r.nextID++
	r.users = append(r.users, user)
	return nil
}

func (r *inMemoryRepo) FindByID(_ context.Context, id int64) (*account.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, u := range r.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, nil
}

func (r *inMemoryRepo) FindByEmail(_ context.Context, email string) (*account.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, u := range r.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, nil
}

// ---------------------------------------------------------------------------
// Health endpoint
// ---------------------------------------------------------------------------

// healthResponse is the JSON response for the health check endpoint.
type healthResponse struct {
	Status string `json:"status"`
	Time   string `json:"time"`
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status: "ok",
		Time:   time.Now().UTC().Format(time.RFC3339),
	})
}

// ---------------------------------------------------------------------------
// Greeting module -- demonstrates the broth.Module pattern
// ---------------------------------------------------------------------------

// greetingModule is a simple module that manages greetings.
// It implements broth.Module (Name + Routes) and broth.OnStart for lifecycle hooks.
type greetingModule struct {
	handler *greetingHandler
}

func newGreetingModule(logger *brothlog.Logger) *greetingModule {
	svc := &greetingService{
		greetings: []greeting{{Name: "Broth", Message: "Welcome to Broth framework!"}},
	}
	h := &greetingHandler{svc: svc, logger: logger}
	return &greetingModule{handler: h}
}

func (m *greetingModule) Name() string           { return "greeting" }
func (m *greetingModule) Routes() []router.Route { return m.handler.routes() }

// Start is called when the app starts (implements broth.OnStart).
func (m *greetingModule) Start(_ context.Context) error { return nil }

// Compile-time interface checks.
var (
	_ broth.Module  = (*greetingModule)(nil)
	_ broth.OnStart = (*greetingModule)(nil)
)

// ---------------------------------------------------------------------------
// Greeting handler (HTTP layer -- presentation only)
// ---------------------------------------------------------------------------

type greetingHandler struct {
	svc    *greetingService
	logger *brothlog.Logger
}

func (h *greetingHandler) routes() []router.Route {
	return []router.Route{
		{Pattern: "GET /", Handler: http.HandlerFunc(h.list)},
		{Pattern: "GET /{name}", Handler: http.HandlerFunc(h.show)},
		{Pattern: "POST /", Handler: http.HandlerFunc(h.create)},
	}
}

// list returns all greetings as JSON.
// GET /greeting/
func (h *greetingHandler) list(w http.ResponseWriter, r *http.Request) {
	greetings := h.svc.List(r.Context())
	writeJSON(w, http.StatusOK, greetingListResponse{
		Greetings: greetings,
		Count:     len(greetings),
	})
}

// show returns a personalized greeting.
// GET /greeting/{name}
func (h *greetingHandler) show(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	g, err := h.svc.Greet(r.Context(), name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, g)
}

// create adds a new greeting.
// POST /greeting/
func (h *greetingHandler) create(w http.ResponseWriter, r *http.Request) {
	var input createGreetingInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON"})
		return
	}

	g, err := h.svc.Create(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, errorResponse{Error: err.Error()})
		return
	}

	h.logger.Info(r.Context(), "greeting created", "name", g.Name)
	writeJSON(w, http.StatusCreated, g)
}

// ---------------------------------------------------------------------------
// Greeting service (application layer -- business logic)
// ---------------------------------------------------------------------------

const maxNameLength = 100

type greetingService struct {
	mu        sync.RWMutex
	greetings []greeting
}

func (s *greetingService) List(_ context.Context) []greeting {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Defensive copy to prevent caller from mutating internal state.
	result := make([]greeting, len(s.greetings))
	copy(result, s.greetings)
	return result
}

func (s *greetingService) Greet(_ context.Context, name string) (greeting, error) {
	if name == "" {
		return greeting{}, fmt.Errorf("greeting: greet: %w",
			&validationError{field: "name", msg: "name is required"})
	}
	if len(name) > maxNameLength {
		return greeting{}, fmt.Errorf("greeting: greet: %w",
			&validationError{field: "name", msg: "name too long"})
	}
	return greeting{
		Name:    name,
		Message: "Hello, " + name + "! Welcome to Broth.",
	}, nil
}

func (s *greetingService) Create(_ context.Context, input createGreetingInput) (greeting, error) {
	if input.Name == "" {
		return greeting{}, fmt.Errorf("greeting: create: %w",
			&validationError{field: "name", msg: "name is required"})
	}
	if len(input.Name) > maxNameLength {
		return greeting{}, fmt.Errorf("greeting: create: %w",
			&validationError{field: "name", msg: "name too long"})
	}

	g := greeting{
		Name:    input.Name,
		Message: "Hello, " + input.Name + "!",
	}

	s.mu.Lock()
	s.greetings = append(s.greetings, g)
	s.mu.Unlock()

	return g, nil
}

// ---------------------------------------------------------------------------
// Greeting model (domain layer -- no external dependencies)
// ---------------------------------------------------------------------------

type greeting struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

type createGreetingInput struct {
	Name string `json:"name"`
}

type validationError struct {
	field string
	msg   string
}

func (e *validationError) Error() string {
	return e.field + ": " + e.msg
}

// ---------------------------------------------------------------------------
// Response types (typed JSON responses -- no interface{}/any)
// ---------------------------------------------------------------------------

type greetingListResponse struct {
	Greetings []greeting `json:"greetings"`
	Count     int        `json:"count"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func parseLogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
