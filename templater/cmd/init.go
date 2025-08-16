package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/application"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/application/commands"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/application/query"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/build"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/certs"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/client/builder"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/config"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/db/repo"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/dns"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/presentation/rest"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/presentation/scheduler"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/storage"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Init() {
	provisionConfig := config.NewProvisionConfig()
	domainContact := dns.NewDomainContact()

	// DB
	dbConfig := dbs.NewConfig()
	pool, err := pgxpool.New(context.Background(), dbConfig.GetDSN())
	if err != nil {
		log.Fatalf("failed to create pool: %v", err)
	}
	uowFactory := dbs.NewUoWFactory(pool)
	eventRepo := repo.NewEventRepo()
	provisionRepo := repo.NewProvisionRepo()

	// FE Build
	templateBuild := build.NewTemplateBuild()

	// AWS
	cfg, err := awsConfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Panic("can't load aws config", err)
	}
	s3 := storage.NewStorage(cfg)
	dnsProvisioner := dns.NewDNSProvisioner(cfg, domainContact)
	acmCerts := certs.NewACMCertificates(cfg)

	builderClient := builder.NewBuilderClient(builder.NewBuilderConfig())

	commands := &application.Commands{
		ProvisionSite:     commands.NewProvisionSite(provisionConfig, uowFactory, eventRepo, provisionRepo, s3, templateBuild, dnsProvisioner, acmCerts),
		ProvisionCDN:      commands.NewProvisionCDN(provisionConfig, uowFactory, provisionRepo, dnsProvisioner),
		FinalizeProvision: commands.NewFinalizeProvision(provisionConfig, uowFactory, dnsProvisioner, builderClient),
		RequestProvision:  commands.NewRequestProvision(uowFactory),
		CheckDomain:       query.NewCheckDomain(dnsProvisioner),
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

	uowFactory.Pool.Close()
	fmt.Println("Fiber was successfully shutdown.")
}
