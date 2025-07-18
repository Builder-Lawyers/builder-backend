package rest

import (
	"builder-templater/internal/application"
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
	var req ProvisionSiteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: err.Error()})
	}

	provisionedSiteID, err := s.commands.ProvisionSite.Execute(req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: err.Error()})
	}
	resp := ProvisionSiteResponse{
		SiteID: provisionedSiteID,
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}
