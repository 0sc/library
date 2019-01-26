package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/boltdb/bolt"
	"github.com/go-chi/chi"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
)

var commentables = []string{"authors", "books"}

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer logger.Sync()

	var cfg config
	err = envconfig.Process("", &cfg)
	if err != nil {
		logger.Fatal("failed to process env vars", zap.Error(err))
	}

	db, err := bolt.Open(cfg.DSN, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		logger.Fatal("failed to setup db", zap.Error(err))
	}

	svc := newService(db, logger)
	err = svc.setup(commentables)
	if err != nil {
		logger.Fatal("failed to setup commentables", zap.Error(err), zap.Any("commentables", commentables))
	}

	router := chi.NewMux()
	svc.registerRoutes(router)

	server := &http.Server{
		Handler: router,
		Addr:    fmt.Sprintf(":%d", cfg.Port),
	}

	logger.Info("starting service", zap.Int("port", cfg.Port))
	go prepareGracefulShutdown(logger, server)

	err = server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		logger.Fatal("http server error occurred", zap.Error(err))
	}

	logger.Info("service shutdown successful")
}

func prepareGracefulShutdown(logger *zap.Logger, srv *http.Server) {
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-signalChannel

	// allow 15 seconds to shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("failed to shutdown server gracefully", zap.Error(err))
	}
}
