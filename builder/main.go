package main

import (
	"builder/internal/application/command"
	"builder/internal/infra/client/templater"
	"builder/internal/infra/db"
	"builder/internal/presentation/rest"
	"context"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

//go:generate go tool oapi-codegen -config .\api\cfg.yaml .\api\openapi.yaml
func main() {
	//build.RunFrontendBuild("./test-task")
	//s3 := storage.NewStorage()
	//s3.ListFiles()

	// Example setup (replace with your real UOW initialization)
	dbConfig := db.NewConfig() // hypothetical constructor
	uowFactory := db.NewUoWFactory(db.New(dbConfig))
	createSite := command.NewCreateSite(*uowFactory, templater.TemplaterClient{})
	commands := command.Collection{
		CreateSite: createSite,
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

	c := make(chan os.Signal, 1)                    // Create channel to signify a signal being sent
	signal.Notify(c, os.Interrupt, syscall.SIGTERM) // When an interrupt or termination signal is sent, notify the channel

	_ = <-c // This blocks the main thread until an interrupt is received
	fmt.Println("Gracefully shutting down...")
	_ = app.Shutdown()

	fmt.Println("Running cleanup tasks...")

	// Your cleanup tasks go here
	// db.Close()
	uowFactory.Conn.Close(context.Background())
	//redisConn.Close()
	fmt.Println("Fiber was successful shutdown.")
}
