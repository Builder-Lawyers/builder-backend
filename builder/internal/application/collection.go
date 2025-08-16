package application

import (
	"github.com/Builder-Lawyers/builder-backend/builder/internal/application/commands"
	"github.com/Builder-Lawyers/builder-backend/builder/internal/application/query"
)

type Collection struct {
	*commands.CreateSite
	*commands.UpdateSite
	*commands.DeleteSite
	*commands.EnrichContent
	*query.GetSite
}
