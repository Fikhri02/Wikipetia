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

	"github.com/Fikhri02/wikipetia/internal/platform/config"
)

type healthOutput struct {
	Body struct {
		Status string `json:"status"`
	}
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

func newAPI() (http.Handler, huma.API) {
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("WikiPetia", "0.1.0"))
	registerHealth(api)
	return mux, api
}

func main() {
	dumpSpec := flag.Bool("dump-openapi", false, "print the OpenAPI spec and exit")
	flag.Parse()

	handler, api := newAPI()

	if *dumpSpec {
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

	addr := fmt.Sprintf(":%d", cfg.Port)
	slog.Info("listening", "addr", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		slog.Error("server", "err", err)
		os.Exit(1)
	}
}
