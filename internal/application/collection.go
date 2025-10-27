package application

import (
	"github.com/Builder-Lawyers/builder-backend/internal/application/commands/ai"
	"github.com/Builder-Lawyers/builder-backend/internal/application/commands/auth"
	"github.com/Builder-Lawyers/builder-backend/internal/application/commands/file"
	"github.com/Builder-Lawyers/builder-backend/internal/application/commands/payment"
	"github.com/Builder-Lawyers/builder-backend/internal/application/commands/site"
	"github.com/Builder-Lawyers/builder-backend/internal/application/commands/template"
	"github.com/Builder-Lawyers/builder-backend/internal/application/processors"
	"github.com/Builder-Lawyers/builder-backend/internal/application/query"
	authCfg "github.com/Builder-Lawyers/builder-backend/internal/infra/auth"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/build"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/certs"
	aiCfg "github.com/Builder-Lawyers/builder-backend/internal/infra/client/openai"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/config"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/dns"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/mail"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/storage"
	"github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
)

type Handlers struct {
	Commands   *Commands
	Queries    *Queries
	Processors *Processors
}

type Commands struct {
	EnrichContent  *ai.EnrichContent
	Auth           *auth.Auth
	UploadFile     *file.UploadFile
	Payment        *payment.Payment
	CreateSite     *site.CreateSite
	UpdateSite     *site.UpdateSite
	DeleteSite     *site.DeleteSite
	CreateTemplate *template.CreateTemplate
}

type Queries struct {
	GetSite     *query.GetSite
	CheckDomain *query.CheckDomain
	GetTemplate *query.GetTemplate
}

type Processors struct {
	DeactivateSite    *processors.DeactivateSite
	ProvisionSite     *processors.ProvisionSite
	ProvisionCDN      *processors.ProvisionCDN
	FinalizeProvision *processors.FinalizeProvision
	SendMail          *processors.SendMail
}

func NewCommands(uowFactory *db.UOWFactory, storage *storage.Storage, uploadConfig file.UploadConfig,
	paymentConfig payment.PaymentConfig, oidcConfig authCfg.OIDCConfig, cognito *cognitoidentityprovider.Client,
) *Commands {
	return &Commands{
		EnrichContent:  ai.NewEnrichContent(aiCfg.NewOpenAIClient(aiCfg.NewOpenAIConfig())),
		Auth:           auth.NewAuth(uowFactory, oidcConfig, cognito),
		UploadFile:     file.NewUploadFile(uowFactory, storage, uploadConfig),
		Payment:        payment.NewPayment(uowFactory, paymentConfig),
		CreateSite:     site.NewCreateSite(uowFactory),
		UpdateSite:     site.NewUpdateSite(uowFactory),
		DeleteSite:     site.NewDeleteSite(uowFactory),
		CreateTemplate: template.NewCreateTemplate(uowFactory),
	}
}

func NewQueries(uowFactory *db.UOWFactory, storage *storage.Storage, provisionConfig config.ProvisionConfig,
	dnsProvisioner *dns.DNSProvisioner,
) *Queries {
	return &Queries{
		GetSite:     query.NewGetSite(provisionConfig, uowFactory, dnsProvisioner),
		CheckDomain: query.NewCheckDomain(dnsProvisioner),
		GetTemplate: query.NewGetTemplate(uowFactory, storage, provisionConfig),
	}
}

func NewProcessors(uowFactory *db.UOWFactory, storage *storage.Storage, build *build.TemplateBuild,
	certs *certs.ACMCertificates, provisionConfig config.ProvisionConfig, dnsProvisioner *dns.DNSProvisioner, mail *mail.MailServer,
) *Processors {
	return &Processors{
		DeactivateSite:    processors.NewDeactivateSite(uowFactory, dnsProvisioner, provisionConfig),
		ProvisionSite:     processors.NewProvisionSite(provisionConfig, uowFactory, storage, build, dnsProvisioner, certs),
		ProvisionCDN:      processors.NewProvisionCDN(provisionConfig, uowFactory, dnsProvisioner),
		FinalizeProvision: processors.NewFinalizeProvision(provisionConfig, uowFactory, dnsProvisioner),
		SendMail:          processors.NewSendMail(mail, uowFactory),
	}
}
