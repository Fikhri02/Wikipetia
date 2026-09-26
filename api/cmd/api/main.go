package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Fikhri02/wikipetia/internal/platform/auth"
	"github.com/Fikhri02/wikipetia/internal/platform/config"
	"github.com/Fikhri02/wikipetia/internal/platform/db"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
	"github.com/Fikhri02/wikipetia/internal/user"
)

type healthOutput struct {
	Body struct {
		Status string `json:"status"`
	}
}

type server struct {
	runner db.Runner
}

func registerHealth(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "health",
		Method:      http.MethodGet,
		Path:        "/health",
		Summary:     "Liveness probe",
	}, func(ctx context.Context, _ *struct{}) (*healthOutput, error) {
		out := &healthOutput{}
		out.Body.Status = "ok"
		return out, nil
	})
}

func (s *server) registerRoutes(api huma.API) {
	s.registerCourseRoutes(api)
	s.registerStructureRoutes(api)
	s.registerBlockRoutes(api)
	s.registerQuestionRoutes(api)
	s.registerOutlineRoutes(api)
	s.registerEnrolmentRoutes(api)
	s.registerProgressRoutes(api)
	s.registerSubmitRoute(api)
	s.registerPlayerRoutes(api)
}

func newAPI(pool *pgxpool.Pool, devUserName string) (http.Handler, huma.API) {
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("WikiPetia", "0.1.0"))

	registerHealth(api)

	// Routes are ALWAYS registered, even with no pool: -dump-openapi runs
	// without a database and the generated spec must still describe the whole
	// API. Handlers are never invoked on that path, so the nil runner is safe.
	var runner db.Runner
	if pool != nil {
		runner = db.PoolRunner{Pool: pool}
	}
	srv := &server{runner: runner}
	srv.registerRoutes(api)

	if pool == nil {
		// Spec-dump path: no database, so no auth middleware either.
		return mux, api
	}

	resolve := func(ctx context.Context, subject, name string) (uuid.UUID, error) {
		var id uuid.UUID
		err := db.WithTx(ctx, pool, func(q *sqlcgen.Queries) error {
			var err error
			id, err = user.Store{}.EnsureBySubject(ctx, q, subject, name)
			return err
		})
		return id, err
	}

	handler := auth.Middleware(mux, auth.DevStub{DefaultName: devUserName}, resolve)
	return handler, api
}

func main() {
	dumpSpec := flag.Bool("dump-openapi", false, "print the OpenAPI spec and exit")
	flag.Parse()

	if *dumpSpec {
		_, api := newAPI(nil, "dev")
		spec, err := api.OpenAPI().YAML()
		if err != nil {
			slog.Error("dump openapi", "err", err)
			os.Exit(1)
		}
		fmt.Print(string(spec))
		return
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	handler, _ := newAPI(pool, cfg.DevUserName)
	addr := fmt.Sprintf(":%d", cfg.Port)
	slog.Info("listening", "addr", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		slog.Error("server", "err", err)
		os.Exit(1)
	}
}
