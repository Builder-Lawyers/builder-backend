package application

import (
	"github.com/Builder-Lawyers/builder-backend/templater/internal/application/commands"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/application/query"
)

type Commands struct {
	*commands.ProvisionSite
	*commands.RequestProvision
	*commands.ProvisionCDN
	*commands.FinalizeProvision
	*query.CheckDomain
}
