package cmd

import (
	"context"
	"fmt"
	"github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/application"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/build"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/certs"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/client/builder"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/config"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/dns"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/presentation/rest"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/presentation/scheduler"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/storage"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/gofiber/fiber/v2"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func Init() {
	// DB
	dbConfig := db.NewConfig()
	uowFactory := db.NewUoWFactory(db.New(dbConfig))

	// FE Build
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println("Current working directory:", wd)
	parent := filepath.Dir(filepath.Dir(wd))
	target := filepath.Join(parent, "production", "template")
	//target := filepath.Join(parent, "templates")
	fmt.Println(target)
	templateBuild := build.NewTemplateBuild(target)

	// AWS
	cfg, err := awsConfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Panic("can't load aws config", err)
	}
	s3 := storage.NewStorage(cfg)
	dnsProvisioner := dns.NewDNSProvisioner(cfg)
	acmCerts := certs.NewACMCertificates(cfg)

	provisionConfig := &config.ProvisionConfig{
		BuildFolder: target,
		PathToFile:  "/src/pages/",
		Filename:    "_page.json",
	}

	builderClient := builder.NewBuilderClient(builder.NewBuilderConfig())

	commands := application.Commands{
		ProvisionSite:    *application.NewProvisionSite(provisionConfig, uowFactory, s3, templateBuild, dnsProvisioner, acmCerts, builderClient),
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
