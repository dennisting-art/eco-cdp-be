package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/internal/auth"
	controller "gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/internal/handlers/http/controller"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/internal/handlers/http/router"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/internal/repository"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/internal/selfpilot"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/internal/service"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/database"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	for _, key := range []string{"DATABASE_URL", "AUTH_ISSUER", "AUTH_JWKS_URL", "AUTH_AUDIENCE", "AUTH_INTROSPECTION_URL"} {
		if os.Getenv(key) == "" {
			return errors.New("missing " + key)
		}
	}
	authConfig := auth.Config{Issuer: os.Getenv("AUTH_ISSUER"), JWKSURL: os.Getenv("AUTH_JWKS_URL"), Audience: os.Getenv("AUTH_AUDIENCE")}
	if err := auth.ValidateConfig(authConfig); err != nil {
		return err
	}
	if err := auth.ValidateIntrospectionURL(os.Getenv("AUTH_INTROSPECTION_URL")); err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return err
	}
	ttl := 30 * time.Second
	if value := os.Getenv("AUTH_CACHE_TTL"); value != "" {
		ttl, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
	}
	authorizer, err := auth.NewAuthorizer(auth.NewValidator(authConfig), &auth.IntrospectionClient{URL: os.Getenv("AUTH_INTROSPECTION_URL")}, ttl)
	if err != nil {
		return err
	}
	var pilot *selfpilot.Service
	if raw := os.Getenv("SELF_PILOT_ENABLED"); raw != "" {
		enabled, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		if enabled {
			pilotTTL := time.Hour
			if value := os.Getenv("SELF_PILOT_TOKEN_TTL"); value != "" {
				pilotTTL, err = time.ParseDuration(value)
				if err != nil {
					return err
				}
			}
			pilot, err = selfpilot.New(os.Getenv("APP_ENV"), selfpilot.Config{Enabled: true, Issuer: os.Getenv("SELF_PILOT_ISSUER"), TokenTTL: pilotTTL})
			if err != nil {
				return err
			}
		}
	}
	db := &database.PostgresDB{Pool: pool}
	handler, err := router.New(router.Dependencies{
		Auth:     authorizer,
		Cards:    controller.NewCardsHandler(service.NewCardsService(repository.NewCardsRepository(db))),
		Universe: controller.NewUniverseHandler(service.NewUniverseService(repository.NewUniverseRepository(db))),
		DB:       pool, Pilot: pilot,
	})
	if err != nil {
		return err
	}
	address := os.Getenv("HTTP_ADDR")
	if address == "" {
		address = ":8080"
	}
	server := &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: time.Minute}
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
