package application

import (
	"github.com/Builder-Lawyers/builder-backend/internal/application/commands"
	"github.com/Builder-Lawyers/builder-backend/internal/application/query"
)

type Handlers struct {
	*commands.CreateSite
	*commands.UpdateSite
	*commands.DeleteSite
	*commands.EnrichContent
	*commands.ProvisionSite
	*commands.ProvisionCDN
	*commands.FinalizeProvision
	*commands.DeactivateSite
	*commands.SendMail
	*commands.Auth
	*commands.Payment
	*query.GetSite
	*query.CheckDomain
	*query.GetTemplate
}
