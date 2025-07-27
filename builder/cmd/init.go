package cmd

import (
	"context"
	"fmt"
	"github.com/Builder-Lawyers/builder-backend/builder/internal/application/command"
	ai "github.com/Builder-Lawyers/builder-backend/builder/internal/infra/client/openai"
	"github.com/Builder-Lawyers/builder-backend/builder/internal/infra/client/templater"
	"github.com/Builder-Lawyers/builder-backend/builder/internal/presentation/rest"
	"github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/gofiber/fiber/v2"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func Init() {
	dbConfig := db.NewConfig()
	uowFactory := db.NewUoWFactory(db.New(dbConfig))
	templaterClient := templater.NewTemplaterClient(templater.NewTemplaterConfig())
	commands := command.Collection{
		CreateSite:    command.NewCreateSite(uowFactory),
		UpdateSite:    command.NewUpdateSite(uowFactory, *templaterClient),
		EnrichContent: command.NewEnrichContent(ai.NewOpenAIClient(ai.NewOpenAIConfig())),
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

	uowFactory.Conn.Close(context.Background())
	fmt.Println("Fiber was successfully shutdown.")
}
