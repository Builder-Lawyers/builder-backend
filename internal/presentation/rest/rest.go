package rest

import (
	"github.com/Builder-Lawyers/builder-backend/internal/application"
	"github.com/Builder-Lawyers/builder-backend/internal/application/dto"
	"github.com/gofiber/fiber/v2"
)

var _ ServerInterface = (*Server)(nil)

type Server struct {
	handlers *application.Handlers
}

func NewServer(handlers *application.Handlers) *Server {
	return &Server{handlers: handlers}
}

func (s Server) CreateSite(c *fiber.Ctx) error {
	var req dto.CreateSiteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	siteID, err := s.handlers.CreateSite.Execute(req)
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

	updatedSiteID, err := s.handlers.UpdateSite.Execute(id, req)
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

	enrichContent, err := s.handlers.EnrichContent.Execute(req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	resp := dto.EnrichContentResponse{
		Enriched: enrichContent,
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (s *Server) CheckDomain(c *fiber.Ctx) error {
	var req dto.CheckDomainParams
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	available, err := s.handlers.CheckDomain.Query(req.Domain)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}
	resp := dto.DomainAvailability{
		Available: available,
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (s Server) GetSite(c *fiber.Ctx, id uint64) error {
	resp, err := s.handlers.GetSite.Query(id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (s Server) GetToken(c *fiber.Ctx) error {
	var req dto.AuthCode
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	accessToken, err := s.handlers.Auth.Callback(req.Code)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	resp := dto.AccessToken{
		Token: accessToken,
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}
