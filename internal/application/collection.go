package application

import (
	"github.com/Builder-Lawyers/builder-backend/internal/application/commands"
	"github.com/Builder-Lawyers/builder-backend/internal/application/commands/auth"
	"github.com/Builder-Lawyers/builder-backend/internal/application/query"
)

type Handlers struct {
	*commands.CreateSite
	*commands.UpdateSite
	*commands.DeleteSite
	*commands.CreateTemplate
	*commands.EnrichContent
	*commands.UploadFile
	*commands.ProvisionSite
	*commands.ProvisionCDN
	*commands.FinalizeProvision
	*commands.DeactivateSite
	*commands.SendMail
	*auth.Auth
	*commands.Payment
	*query.GetSite
	*query.CheckDomain
	*query.GetTemplate
}
