package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application"
	"github.com/Builder-Lawyers/builder-backend/internal/application/commands"
	"github.com/Builder-Lawyers/builder-backend/internal/application/query"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/build"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/certs"
	ai "github.com/Builder-Lawyers/builder-backend/internal/infra/client/openai"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/config"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db/repo"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/dns"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/mail"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/storage"
	"github.com/Builder-Lawyers/builder-backend/internal/presentation/rest"
	"github.com/Builder-Lawyers/builder-backend/internal/presentation/scheduler"
	"github.com/Builder-Lawyers/builder-backend/pkg/db"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Init() {
	// DB
	dbConfig := db.NewConfig()
	pool, err := pgxpool.New(context.Background(), dbConfig.GetDSN())
	if err != nil {
		log.Fatalf("failed to create pool: %v", err)
	}
	uowFactory := db.NewUoWFactory(pool)

	// Repos
	eventRepo := repo.NewEventRepo()
	provisionRepo := repo.NewProvisionRepo()

	// FE Build
	templateBuild := build.NewTemplateBuild()

	// Configs
	provisionConfig := config.NewProvisionConfig()
	domainContact := dns.NewDomainContact()
	mailConfig := mail.NewMailConfig()
	mailServer := mail.NewMailServer(mailConfig)

	// AWS
	cfg, err := awsConfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Panic("can't load aws config", err)
	}
	s3 := storage.NewStorage(cfg)
	dnsProvisioner := dns.NewDNSProvisioner(cfg, domainContact)
	acmCerts := certs.NewACMCertificates(cfg)

	handlers := &application.Handlers{
		CreateSite:        commands.NewCreateSite(uowFactory),
		UpdateSite:        commands.NewUpdateSite(uowFactory, eventRepo),
		EnrichContent:     commands.NewEnrichContent(ai.NewOpenAIClient(ai.NewOpenAIConfig())),
		GetSite:           query.NewGetSite(provisionConfig, uowFactory, provisionRepo, dnsProvisioner),
		ProvisionSite:     commands.NewProvisionSite(provisionConfig, uowFactory, eventRepo, provisionRepo, s3, templateBuild, dnsProvisioner, acmCerts),
		ProvisionCDN:      commands.NewProvisionCDN(provisionConfig, uowFactory, provisionRepo, dnsProvisioner),
		FinalizeProvision: commands.NewFinalizeProvision(provisionConfig, uowFactory, dnsProvisioner, eventRepo),
		CheckDomain:       query.NewCheckDomain(dnsProvisioner),
		SendMail:          commands.NewSendMail(mailServer, uowFactory),
	}
	handler := rest.NewServer(handlers)
	app := fiber.New(fiber.Config{
		IdleTimeout: 5 * time.Second,
	})
	rest.RegisterHandlers(app, handler)

	outboxPoller := scheduler.NewOutboxPoller(handlers, uowFactory, 5, 5)
	go outboxPoller.Start()

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
