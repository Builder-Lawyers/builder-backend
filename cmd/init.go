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
	"github.com/Builder-Lawyers/builder-backend/internal/application/commands/file"
	"github.com/Builder-Lawyers/builder-backend/internal/application/commands/payment"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/auth"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/build"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/certs"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/config"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/dns"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/mail"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/storage"
	"github.com/Builder-Lawyers/builder-backend/internal/presentation/queue"
	"github.com/Builder-Lawyers/builder-backend/internal/presentation/rest"
	"github.com/Builder-Lawyers/builder-backend/internal/presentation/scheduler"
	"github.com/Builder-Lawyers/builder-backend/pkg/db"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
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

	// Configs
	provisionConfig := config.NewProvisionConfig()
	domainContact := dns.NewDomainContact()
	mailConfig := mail.NewMailConfig()
	oidcConfig := auth.NewOIDCConfig()
	paymentConfig := payment.NewPaymentConfig()
	uploadConfig := file.NewUploadConfig()
	outboxConfig := scheduler.NewOutboxConfig()
	templateChangesConfig := queue.NewTemplateChangesConfig()
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
	cognito := cognitoidentityprovider.NewFromConfig(cfg, func(o *cognitoidentityprovider.Options) {
		o.Region = "us-east-1"
	})
	sqsClient := sqs.NewFromConfig(cfg, func(o *sqs.Options) {
		o.Region = templateChangesConfig.SqsRegion
	})

	// FE Build
	templateBuild := build.NewTemplateBuild(s3, provisionConfig)

	handlers := &application.Handlers{
		Commands:   application.NewCommands(uowFactory, s3, uploadConfig, templateBuild, provisionConfig, paymentConfig, oidcConfig, cognito),
		Queries:    application.NewQueries(uowFactory, s3, provisionConfig, dnsProvisioner),
		Processors: application.NewProcessors(uowFactory, s3, templateBuild, acmCerts, provisionConfig, dnsProvisioner, mailServer),
	}
	handler := rest.NewServer(handlers.Queries, handlers.Commands)
	app := fiber.New(fiber.Config{
		IdleTimeout: 5 * time.Second,
	})
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:3000",
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,Cookie",
		ExposeHeaders:    "Set-Cookie,Authorization",
		AllowCredentials: true,
	}))
	app.Static("/docs", "./api")
	rest.RegisterHandlers(app, handler)

	outboxPoller := scheduler.NewOutboxPoller(handlers.Processors, uowFactory, outboxConfig)
	go outboxPoller.Start()

	templatesQueuePoller := queue.NewTemplateChangesPoller(sqsClient, templateChangesConfig, handlers.Commands.UpdateTemplate)
	if templateChangesConfig.Enabled {
		go templatesQueuePoller.Start()
	}

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
	if templateChangesConfig.Enabled {
		templatesQueuePoller.Stop()
	}

	fmt.Println("Running cleanup tasks...")

	uowFactory.Pool.Close()
	fmt.Println("Fiber was successfully shutdown.")
}
