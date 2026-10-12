// Command api serves the portfolio HTTP API: as a Lambda behind a Function URL in production,
// or as a plain HTTP server locally.
package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/akrylysov/algnhsa"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Routes arrive with the Visualização counter (delivery 5); the spike only needs a deployable Lambda.
	mux := http.NewServeMux()

	if os.Getenv("AWS_LAMBDA_RUNTIME_API") != "" {
		// Function URL events use the API Gateway v2 payload, detected automatically.
		algnhsa.ListenAndServe(mux, nil)
		return
	}

	addr := ":8080"
	slog.Info("listening", slog.String("addr", addr))
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server stopped", slog.Any("error", err))
		os.Exit(1)
	}
}
