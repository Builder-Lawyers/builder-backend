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
	auth2 "github.com/Builder-Lawyers/builder-backend/internal/application/commands/auth"
	"github.com/Builder-Lawyers/builder-backend/internal/application/query"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/auth"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/build"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/certs"
	ai "github.com/Builder-Lawyers/builder-backend/internal/infra/client/openai"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/config"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/dns"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/mail"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/storage"
	"github.com/Builder-Lawyers/builder-backend/internal/presentation/rest"
	"github.com/Builder-Lawyers/builder-backend/internal/presentation/scheduler"
	"github.com/Builder-Lawyers/builder-backend/pkg/db"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/golang-jwt/jwt/v4"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Init() {
	// DB
	dbConfig := db.NewConfig()
	pool, err := pgxpool.New(context.Background(), dbConfig.GetDSN())
	if err != nil {
		log.Panicf("failed to create pool: %v", err)
	}
	err = pool.Ping(context.Background())
	if err != nil {
		log.Panicf("failed to connect to db: %v", err)
	}
	uowFactory := db.NewUoWFactory(pool)
	// FE Build
	templateBuild := build.NewTemplateBuild()

	// Configs
	provisionConfig := config.NewProvisionConfig()
	domainContact := dns.NewDomainContact()
	mailConfig := mail.NewMailConfig()
	oidcConfig := auth.NewOIDCConfig()
	paymentConfig := commands.NewPaymentConfig()
	outboxConfig := scheduler.NewOutboxConfig()
	// solving problem of slight clock mismatch for jwt verifications
	now := time.Now()
	jwt.TimeFunc = func() time.Time {
		return now.Add(60 * time.Second)
	}
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
		UpdateSite:        commands.NewUpdateSite(uowFactory),
		CreateTemplate:    commands.NewCreateTemplate(uowFactory),
		EnrichContent:     commands.NewEnrichContent(ai.NewOpenAIClient(ai.NewOpenAIConfig())),
		UploadFile:        commands.NewUploadFile(uowFactory, s3, "images/"),
		GetSite:           query.NewGetSite(provisionConfig, uowFactory, dnsProvisioner),
		GetTemplate:       query.NewGetTemplate(uowFactory, s3, provisionConfig),
		ProvisionSite:     commands.NewProvisionSite(provisionConfig, uowFactory, s3, templateBuild, dnsProvisioner, acmCerts),
		ProvisionCDN:      commands.NewProvisionCDN(provisionConfig, uowFactory, dnsProvisioner),
		FinalizeProvision: commands.NewFinalizeProvision(provisionConfig, uowFactory, dnsProvisioner),
		DeactivateSite:    commands.NewDeactivateSite(uowFactory, dnsProvisioner),
		Auth:              auth2.NewAuth(uowFactory, oidcConfig, cfg),
		CheckDomain:       query.NewCheckDomain(dnsProvisioner),
		SendMail:          commands.NewSendMail(mailServer, uowFactory),
		Payment:           commands.NewPayment(uowFactory, paymentConfig),
	}
	handler := rest.NewServer(handlers)
	app := fiber.New(fiber.Config{
		IdleTimeout: 5 * time.Second,
	})
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:3000",
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowCredentials: true,
	}))
	app.Static("/docs", "./api")
	rest.RegisterHandlers(app, handler)

	outboxPoller := scheduler.NewOutboxPoller(handlers, uowFactory, outboxConfig)
	go outboxPoller.Start()

	go func() {
		if err := app.Listen(":8080"); err != nil {
			log.Panic(err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	_ = <-c
	fmt.Println("Gracefully shutting down...")
	_ = app.Shutdown()
	outboxPoller.Stop()

	fmt.Println("Running cleanup tasks...")

	uowFactory.Pool.Close()
	fmt.Println("Fiber was successfully shutdown.")
}
