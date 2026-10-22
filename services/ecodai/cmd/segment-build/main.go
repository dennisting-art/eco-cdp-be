package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/database"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/postgres"
)

func main() {
	if os.Getenv("APP_ENV") != "local" {
		fmt.Fprintln(os.Stderr, "segment-build is limited to APP_ENV=local until the organisation guard is enabled")
		os.Exit(2)
	}
	if len(os.Args) != 2 || os.Getenv("DATABASE_URL") == "" {
		fmt.Fprintln(os.Stderr, "usage: DATABASE_URL=... segment-build BUILD_ID")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	pool, err := database.Open(ctx, os.Getenv("DATABASE_URL"))
	if err == nil {
		defer pool.Close()
		err = postgres.NewBuildPublisher(&database.PostgresDB{Pool: pool}).Publish(ctx, os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("published", os.Args[1])
}
