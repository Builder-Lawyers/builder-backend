package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Builder-Lawyers/builder-backend/builder/internal/application"
	"github.com/Builder-Lawyers/builder-backend/builder/internal/application/commands"
	"github.com/Builder-Lawyers/builder-backend/builder/internal/application/query"
	ai "github.com/Builder-Lawyers/builder-backend/builder/internal/infra/client/openai"
	"github.com/Builder-Lawyers/builder-backend/builder/internal/infra/client/templater"
	"github.com/Builder-Lawyers/builder-backend/builder/internal/presentation/rest"
	"github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Init() {
	dbConfig := db.NewConfig()
	pool, err := pgxpool.New(context.Background(), dbConfig.GetDSN())
	if err != nil {
		log.Fatalf("failed to create pool: %v", err)
	}
	uowFactory := db.NewUoWFactory(pool)
	templaterClient := templater.NewTemplaterClient(templater.NewTemplaterConfig())
	commands := &application.Collection{
		CreateSite:    commands.NewCreateSite(uowFactory),
		UpdateSite:    commands.NewUpdateSite(uowFactory, templaterClient),
		EnrichContent: commands.NewEnrichContent(ai.NewOpenAIClient(ai.NewOpenAIConfig())),
		GetSite:       query.NewGetSite(uowFactory, templaterClient),
	}
	handler := rest.NewServer(commands)
	app := fiber.New(fiber.Config{
		IdleTimeout: 5 * time.Second,
	})
	rest.RegisterHandlers(app, handler)

	go func() {
		if err := app.Listen(":3000"); err != nil {
			log.Panic(err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	_ = <-c
	fmt.Println("Gracefully shutting down...")
	_ = app.Shutdown()

	fmt.Println("Running cleanup tasks...")

	uowFactory.Pool.Close()
	fmt.Println("Fiber was successfully shutdown.")
}
