package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pipery-dev/pipery-release-bot/internal/config"
	"github.com/pipery-dev/pipery-release-bot/internal/github"
	"github.com/pipery-dev/pipery-release-bot/internal/httpapi"
	"github.com/pipery-dev/pipery-release-bot/internal/release"
)

func main() {
	cfg, err := config.Load(os.Getenv("PIPERY_RELEASE_CONFIG"))
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	auth := github.NewAppAuthenticator(http.DefaultClient, cfg.Installations)
	client := github.NewClient(http.DefaultClient, auth)
	svc := release.NewService(cfg.Target, cfg.BranchPatterns, client)
	server := httpapi.NewServer(svc, cfg.APIToken)

	addr := cfg.ListenAddr
	if addr == "" {
		addr = ":8080"
	}

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           server.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("pipery-release-bot listening on %s", addr)
		errCh <- httpServer.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-stop:
		log.Printf("shutting down after %s", sig)
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
