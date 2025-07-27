package cmd

import (
	"context"
	"fmt"
	"github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/application"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/build"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/presentation/rest"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/presentation/scheduler"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/storage"
	"github.com/gofiber/fiber/v2"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func Init() {
	dbConfig := db.NewConfig()
	uowFactory := db.NewUoWFactory(db.New(dbConfig))
	s3 := storage.NewStorage()
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println("Current working directory:", wd)
	parent := filepath.Dir(filepath.Dir(wd))
	target := filepath.Join(parent, "templates")
	fmt.Println(target)
	templateBuild := build.NewTemplateBuild(target)
	commands := application.Commands{
		ProvisionSite:    *application.NewProvisionSite(uowFactory, s3, templateBuild, target, "/src/pages/", "_page.json"),
		RequestProvision: *application.NewRequestProvision(uowFactory),
	}
	handler := rest.NewServer(commands)
	app := fiber.New(fiber.Config{
		IdleTimeout: 5 * time.Second,
	})
	rest.RegisterHandlers(app, handler)

	outboxPoller := scheduler.NewOutboxPoller(commands, uowFactory, 5, 5)
	go outboxPoller.Start()

	go func() {
		if err := app.Listen(":3001"); err != nil {
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
