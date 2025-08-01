package rest

import (
	"github.com/Builder-Lawyers/builder-backend/templater/internal/application"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/application/dto"
	"github.com/gofiber/fiber/v2"
)

var _ ServerInterface = (*Server)(nil)

type Server struct {
	commands application.Commands
}

func NewServer(commands application.Commands) Server {
	return Server{commands: commands}
}

func (s Server) ProvisionSite(c *fiber.Ctx) error {
	var req dto.ProvisionSiteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	provisionedSiteID, err := s.commands.RequestProvision.Execute(req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}
	resp := dto.ProvisionSiteResponse{
		SiteID: provisionedSiteID,
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}
