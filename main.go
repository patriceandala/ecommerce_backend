package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/kenshaw/envcfg"
	"github.com/rs/cors"
	"github.com/rs/zerolog"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"
	grpctrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"gopkg.in/DataDog/dd-trace-go.v1/profiler"

	"github.com/dropezy/internal/grpc/interceptors"
	"github.com/dropezy/internal/logging"

	"github.com/dropezy/storefront-backend/internal/storage/mongo"

	"github.com/dropezy/storefront-backend/ems-api/services"
	"github.com/dropezy/storefront-backend/ems-api/services/category"
	"github.com/dropezy/storefront-backend/ems-api/services/product"
)

const service = "ems-api"

var (
	version     = "development"
	environment = "development"

	config *envcfg.Envcfg
	logger zerolog.Logger
)

func init() {
	var err error
	config, err = envcfg.New()
	if err != nil {
		log.Fatal(err)
	}

	environment = config.GetString("runtime.environment")

	logLevel := zerolog.InfoLevel
	levelStr := config.GetString("log.level")
	if levelStr == "fromenv" {
		switch environment {
		case "staging", "development":
			logLevel = zerolog.DebugLevel
		}
	} else {
		var err error
		logLevel, err = zerolog.ParseLevel(levelStr)
		if err != nil {
			log.Fatal(err)
		}
	}

	logger = logging.NewLogger().
		Level(logLevel).With().
		Str("service-name", service).
		Str("version", version).
		Logger()
}

func main() {
	if environment == "production" {
		// TODO(vishen): move datadog tracing and profile stuff to an importable package
		// Start datadog APM
		tracer.Start(
			tracer.WithEnv(environment),
			tracer.WithService(service),
			tracer.WithServiceVersion(version),
			tracer.WithAgentAddr(config.GetString("datadog.agentAddr")),
		)
		defer tracer.Stop()

		err := profiler.Start(
			profiler.WithService(service),
			profiler.WithEnv(environment),
			profiler.WithVersion(version),
			profiler.WithAgentAddr(config.GetString("datadog.agentAddr")),
			profiler.WithProfileTypes(
				profiler.CPUProfile,
				profiler.HeapProfile,
				profiler.GoroutineProfile,
			),
		)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to set up datadog profiler")
		}
		defer profiler.Stop()
	}

	logger.Info().Msgf("starting %s server", service)

	// grpc server init
	grpcServer, err := setupGRPCServer()
	if err != nil {
		logger.Err(err).Msg("failed to setup gRPC server")
	}

	gw, err := setupGrpcGateway(
		category.RegisterGateway,
		product.RegisterGateway,
	)
	if err != nil {
		logger.Err(err).Msg("failed to setup gRPC gateway")
	}

	server := setupServer(grpcServer, gw)

	// empty host because our service will be forwarded to
	// outside via port forwarding.
	addr := net.JoinHostPort("", config.GetString("server.port"))
	l, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Fatal().Err(err).Send()
	}

	go func() {
		logger.Info().Msgf("server listening at %v", l.Addr())
		logger.Fatal().Err(server.Serve(l)).Msg("server shutdown")
	}()

	// catch interrupt signals
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	<-ch
	// perform graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Fatal().Err(err).Msg("failed to shutdown server")
	}
	grpcServer.Stop()

	logger.Info().Msg("server exited gracefully")
}

func setupGRPCServer() (*grpc.Server, error) {
	// initialize mongo client
	mongoStore, err := mongo.NewStorage(config, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("error initializing mongo storage")
	}

	srv := grpc.NewServer(
		interceptors.New(
			logger,
			grpctrace.UnaryServerInterceptor(),
		),
	)

	if err := services.Register(srv,
		category.RegisterService(logger, mongoStore),
		product.RegisterService(logger, mongoStore),
	); err != nil {
		return nil, err
	}

	// health check service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(srv, healthServer)
	// reflection service
	reflection.Register(srv)

	return srv, nil
}

type RegisterGatewayFunc func(ctx context.Context, mux *runtime.ServeMux, addr string, opts []grpc.DialOption) error

func setupGrpcGateway(registerGatewayFuncs ...RegisterGatewayFunc) (http.Handler, error) {
	options := []runtime.ServeMuxOption{
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				UseProtoNames:   true,
				EmitUnpopulated: true,
			},
			UnmarshalOptions: protojson.UnmarshalOptions{
				DiscardUnknown: true,
			},
		}),
	}

	mux := runtime.NewServeMux(options...)
	ctx := context.Background()
	opts := []grpc.DialOption{
		grpc.WithInsecure(),
	}
	addr := net.JoinHostPort("localhost", config.GetString("server.port"))

	for _, r := range registerGatewayFuncs {
		if err := r(ctx, mux, addr, opts); err != nil {
			return nil, fmt.Errorf("unable to register gateway: %w", err)
		}
	}
	return mux, nil
}

// setupServer return http server with h2c handler, it also provide root http route
// to print our server version.
func setupServer(grpcServer *grpc.Server, gw http.Handler) *http.Server {
	return &http.Server{
		Handler: h2c.NewHandler(
			mixedHandler(grpcServer, gw),
			&http2.Server{IdleTimeout: 120 * time.Second},
		),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}

func mixedHandler(grpcServer *grpc.Server, gw http.Handler) http.Handler {
	corsHandler := cors.New(cors.Options{
		AllowCredentials: true,
		AllowedOrigins:   strings.Split(config.GetString("cors.origins"), ","),
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodHead, http.MethodDelete},
		Debug:            true,
	})
	apiMiddleware := &apiMiddleware{
		name:    service,
		version: version,
		auth: &basicAuthMiddleware{
			username: config.GetString("server.username"),
			password: config.GetString("server.password"),
		},
	}
	return apiMiddleware.WrapHandler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodOptions:
				corsHandler.HandlerFunc(w, r)
			case r.ProtoMajor == 2 && r.Header.Get("Content-Type") == "application/grpc":
				grpcServer.ServeHTTP(w, r)
			default:
				corsHandler.ServeHTTP(w, r, gw.ServeHTTP)
			}
		}),
	)
}
