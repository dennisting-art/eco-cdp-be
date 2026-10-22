package main

import (
	"cloud.google.com/go/storage"
	"context"
	"errors"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/logger"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/auth"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/config"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/database"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/api"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/postgres"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/service"
	segmentsapi "gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/api"
	segmentspostgres "gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/postgres"
	segmentsservice "gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/service"
	segmentsworker "gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/worker"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/selfpilot"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/system"
)

func main() {
	if err := run(); err != nil {
		logger.Error(context.Background(), "server stopped", "category", "startup")
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	authorizer, err := auth.NewAuthorizer(auth.NewValidator(cfg.Auth), &auth.IntrospectionClient{URL: cfg.IntrospectionURL}, cfg.AuthCacheTTL)
	if err != nil {
		return err
	}
	var pilot *selfpilot.Service
	if cfg.SelfPilotEnabled {
		pilot, err = selfpilot.New(cfg.Environment, selfpilot.Config{Enabled: true, Issuer: cfg.SelfPilotIssuer, TokenTTL: cfg.SelfPilotTokenTTL})
		if err != nil {
			return err
		}
	}
	db := &database.PostgresDB{Pool: pool}
	var segmentHandler *segmentsapi.Handler
	if cfg.SegmentAPIEnabled {
		var segmentRepo segmentsservice.Repository = segmentspostgres.NewRepository(db)
		var segmentEval segmentsservice.Evaluator
		if cfg.SegmentBuildEnabled {
			segmentRepo = segmentspostgres.NewBuildRepository(db)
			segmentEval = segmentspostgres.NewBuildEvaluator(db)
		}
		segmentHandler = &segmentsapi.Handler{
			Service:     segmentsservice.NewService(segmentRepo, segmentEval, cfg.SegmentWorkerEnabled),
			ListingOnly: !cfg.SegmentBuildEnabled,
		}
	}
	if cfg.SegmentWorkerEnabled {
		var exporter segmentsworker.Exporter
		if cfg.Environment == "local" {
			exporter = segmentsworker.LocalExporter{Root: cfg.SegmentLocalExportDir}
		} else {
			client, err := storage.NewClient(ctx)
			if err != nil {
				return err
			}
			defer client.Close()
			exporter = segmentsworker.GCSExporter{Client: client, Bucket: cfg.SegmentGCSBucket}
		}
		go segmentsworker.NewWorker(db, exporter).Run(ctx)
	}
	handler, err := system.New(system.Dependencies{
		Auth:     authorizer,
		Cards:    api.NewCardsHandler(service.NewCardsService(postgres.NewCardsRepository(db))),
		Universe: api.NewUniverseHandler(service.NewUniverseService(postgres.NewUniverseRepository(db))),
		DB:       pool, Pilot: pilot, Segments: segmentHandler,
	})
	if err != nil {
		return err
	}
	server := &http.Server{Addr: cfg.HTTPAddress, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: time.Minute}
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
