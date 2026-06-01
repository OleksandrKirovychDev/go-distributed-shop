// Command identity is the composition root for the Identity service: it loads
// config, initialises logging/OTel/Postgres, wires the hexagonal adapters into
// the use cases by hand, and serves gRPC (health + reflection) alongside HTTP
// (the federated GraphQL subgraph + JWKS) until a termination signal triggers a
// bounded graceful shutdown.
package main

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/online-shop/pkg/auth"
	"github.com/online-shop/pkg/config"
	pkggrpc "github.com/online-shop/pkg/grpc"
	"github.com/online-shop/pkg/logger"
	"github.com/online-shop/pkg/otel"
	"github.com/online-shop/pkg/postgres"

	identityv1 "github.com/online-shop/proto/gen/go/identity/v1"

	graphqlinbound "github.com/online-shop/services/identity/internal/adapters/inbound/graphql"
	grpcinbound "github.com/online-shop/services/identity/internal/adapters/inbound/grpc"
	"github.com/online-shop/services/identity/internal/adapters/outbound/argon2"
	"github.com/online-shop/services/identity/internal/adapters/outbound/events"
	jwtadapter "github.com/online-shop/services/identity/internal/adapters/outbound/jwt"
	pgadapter "github.com/online-shop/services/identity/internal/adapters/outbound/postgres"
	"github.com/online-shop/services/identity/internal/adapters/outbound/system"
	"github.com/online-shop/services/identity/internal/app/getuser"
	"github.com/online-shop/services/identity/internal/app/loginuser"
	"github.com/online-shop/services/identity/internal/app/refreshtoken"
	"github.com/online-shop/services/identity/internal/app/registeruser"
	"github.com/online-shop/services/identity/internal/app/verifytoken"
)

type Config struct {
	GRPCAddr          string          `envconfig:"GRPC_ADDR" default:":50051"`
	HTTPAddr          string          `envconfig:"HTTP_ADDR" default:":8081"`
	Postgres          postgres.Config `envconfig:"POSTGRES"`
	ServiceVersion    string          `envconfig:"SERVICE_VERSION" default:"dev"`
	Environment       string          `envconfig:"ENVIRONMENT" default:"local"`
	JWTIssuer         string          `envconfig:"JWT_ISSUER" default:"identity"`
	JWTAudience       string          `envconfig:"JWT_AUDIENCE" default:"online-shop"`
	JWTPrivateKeyPath string          `envconfig:"JWT_PRIVATE_KEY_PATH"`
	AccessTokenTTL    time.Duration   `envconfig:"ACCESS_TOKEN_TTL" default:"15m"`
	RefreshTokenTTL   time.Duration   `envconfig:"REFRESH_TOKEN_TTL" default:"720h"`
	ShutdownTimeout   time.Duration   `envconfig:"SHUTDOWN_TIMEOUT" default:"30s"`
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := healthcheck(); err != nil {
			slog.Error("healthcheck failed", "err", err)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		slog.Error("identity failed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	cfg := config.MustLoad[Config]("IDENTITY")
	log := logger.New("identity")

	otelShutdown, err := otel.Init(rootCtx, "identity",
		otel.WithServiceVersion(cfg.ServiceVersion),
		otel.WithEnvironment(cfg.Environment),
	)
	if err != nil {
		return fmt.Errorf("otel init: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if sErr := otelShutdown(shutdownCtx); sErr != nil {
			log.Error("otel shutdown", "err", sErr)
		}
	}()

	pool, err := postgres.NewPool(rootCtx, cfg.Postgres, log)
	if err != nil {
		return fmt.Errorf("postgres pool: %w", err)
	}
	defer pool.Close()

	key, err := loadSigningKey(cfg.JWTPrivateKeyPath, log)
	if err != nil {
		return fmt.Errorf("load signing key: %w", err)
	}

	// Outbound adapters.
	userRepo := pgadapter.NewUserRepository(pool)
	refreshRepo := pgadapter.NewRefreshTokenRepository(pool)
	txManager := pgadapter.NewTxManager(pool)
	encoder := events.NewEncoder()
	hasher := argon2.NewPasswordHasher(argon2.DefaultParams())
	refreshGen := argon2.NewRefreshTokenGenerator()
	clock := system.NewClock()
	idGen := system.NewIDGenerator()
	issuer := jwtadapter.NewTokenIssuer(auth.NewTokenIssuer(key, cfg.JWTIssuer, cfg.JWTAudience, cfg.AccessTokenTTL))
	parser := jwtadapter.NewTokenParser(auth.NewVerifier(&key.PublicKey, cfg.JWTIssuer, cfg.JWTAudience))

	// Use cases.
	registrar := registeruser.New(hasher, encoder, txManager, clock, idGen)
	authenticator := loginuser.New(userRepo, refreshRepo, hasher, issuer, refreshGen, clock, idGen, cfg.RefreshTokenTTL)
	refresher := refreshtoken.New(userRepo, refreshRepo, issuer, refreshGen, txManager, clock, idGen, cfg.RefreshTokenTTL)
	tokenVerifier := verifytoken.New(parser)
	userGetter := getuser.New(userRepo)

	// Inbound gRPC.
	inbound := grpcinbound.NewServer(registrar, authenticator, refresher, tokenVerifier, userGetter)
	grpcServer := pkggrpc.NewServer(log)
	identityv1.RegisterIdentityServiceServer(grpcServer, inbound)
	health := pkggrpc.RegisterHealth(grpcServer)
	health.SetServingStatus("", healthgrpc.HealthCheckResponse_SERVING)
	reflection.Register(grpcServer)

	listener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.GRPCAddr, err)
	}

	// Inbound GraphQL: the federated subgraph plus the JWKS the router fetches to
	// validate access tokens. The router checks the edge JWT and forwards the
	// principal as headers; AuthMiddleware lifts those into the request context.
	es := graphqlinbound.NewExecutableSchema(graphqlinbound.Config{
		Resolvers: graphqlinbound.NewResolver(registrar, authenticator, userGetter),
	})
	gqlServer := handler.New(es)
	gqlServer.AddTransport(transport.POST{})
	gqlServer.Use(extension.Introspection{})

	jwksHandler, err := jwtadapter.NewJWKSHandler(&key.PublicKey)
	if err != nil {
		return fmt.Errorf("jwks handler: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/graphql", otelhttp.NewHandler(graphqlinbound.AuthMiddleware(gqlServer), "identity.graphql"))
	mux.Handle("GET /.well-known/jwks.json", jwksHandler)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serveErr := make(chan error, 2)
	go func() {
		log.Info("identity gRPC server listening", "addr", cfg.GRPCAddr)
		if sErr := grpcServer.Serve(listener); sErr != nil {
			serveErr <- fmt.Errorf("grpc serve: %w", sErr)
		}
	}()
	go func() {
		log.Info("identity HTTP server listening", "addr", cfg.HTTPAddr)
		if sErr := httpServer.ListenAndServe(); sErr != nil && !errors.Is(sErr, http.ErrServerClosed) {
			serveErr <- fmt.Errorf("http serve: %w", sErr)
		}
	}()

	select {
	case sErr := <-serveErr:
		return sErr
	case <-rootCtx.Done():
		log.Info("shutdown signal received, draining in-flight requests")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	// Stop accepting edge traffic first, then drain in-flight RPCs on the same
	// deadline.
	if sErr := httpServer.Shutdown(shutdownCtx); sErr != nil {
		log.Error("http graceful shutdown", "err", sErr)
	}

	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-shutdownCtx.Done():
		log.Warn("graceful shutdown deadline exceeded, forcing stop")
		grpcServer.Stop()
	}

	log.Info("identity stopped")
	return nil
}

// loadSigningKey reads the RSA signing key from a PEM file, or mints an ephemeral
// one when no path is configured. The ephemeral key's kid changes on every
// restart, so it only suits a single-process dev run where signer and verifier
// are the same process; a JWKS-published deployment must supply a durable key.
func loadSigningKey(path string, log *slog.Logger) (*rsa.PrivateKey, error) {
	if path == "" {
		log.Warn("no signing key configured; generating an ephemeral key (dev only, kid changes per restart)")
		key, err := auth.GenerateEphemeralKey()
		if err != nil {
			return nil, fmt.Errorf("generate ephemeral key: %w", err)
		}
		return key, nil
	}

	data, err := os.ReadFile(path) //nolint:gosec // G304: operator-supplied key path from config, not user input
	if err != nil {
		return nil, fmt.Errorf("read signing key %s: %w", path, err)
	}
	key, err := auth.LoadPrivateKeyFromPEM(data)
	if err != nil {
		return nil, fmt.Errorf("parse signing key %s: %w", path, err)
	}
	return key, nil
}

// healthcheck probes the local HTTP /healthz endpoint, exiting non-zero on
// failure. It backs the container HEALTHCHECK: the distroless image has no shell
// or curl, so the binary checks its own liveness when invoked as `identity
// healthcheck`.
func healthcheck() error {
	addr := os.Getenv("IDENTITY_HTTP_ADDR")
	if addr == "" {
		addr = ":8081"
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("parse HTTP addr %q: %w", addr, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	//nolint:gosec // G704: loopback self-probe to our own configured port, not user input.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+port+"/healthz", http.NoBody)
	if err != nil {
		return fmt.Errorf("new healthcheck request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req) //nolint:gosec // G704: see above; loopback self-probe.
	if err != nil {
		return fmt.Errorf("get healthz: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthz status %d", resp.StatusCode)
	}
	return nil
}
