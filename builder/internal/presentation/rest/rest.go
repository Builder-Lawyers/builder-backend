package rest

import (
	"github.com/Builder-Lawyers/builder-backend/builder/internal/application/command"
	"github.com/Builder-Lawyers/builder-backend/builder/internal/application/dto"
	"github.com/gofiber/fiber/v2"
)

var _ ServerInterface = (*Server)(nil)

type Server struct {
	commands command.Collection
}

func NewServer(commands command.Collection) Server {
	return Server{commands: commands}
}

func (s Server) CreateSite(c *fiber.Ctx) error {
	var req dto.CreateSiteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	siteID, err := s.commands.CreateSite.Execute(req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	resp := dto.CreateSiteResponse{
		SiteID: siteID,
	}

	return c.Status(fiber.StatusCreated).JSON(resp)
}

func (s Server) UpdateSite(c *fiber.Ctx, id uint64) error {
	var req dto.UpdateSiteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	updatedSiteID, err := s.commands.UpdateSite.Execute(id, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	resp := dto.UpdateSiteResponse{
		SiteID: updatedSiteID,
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (s Server) EnrichContent(c *fiber.Ctx) error {
	var req dto.EnrichContentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	enrichContent, err := s.commands.EnrichContent.Execute(req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	resp := dto.EnrichContentResponse{
		Enriched: enrichContent,
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}
