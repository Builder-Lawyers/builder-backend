package rest

import (
	"log/slog"
	"strings"

	"github.com/Builder-Lawyers/builder-backend/internal/application"
	"github.com/Builder-Lawyers/builder-backend/internal/application/dto"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/auth"
	"github.com/gofiber/fiber/v2"
)

var _ ServerInterface = (*Server)(nil)

type Server struct {
	handlers *application.Handlers
}

func NewServer(handlers *application.Handlers) *Server {
	return &Server{handlers: handlers}
}

func (s *Server) CreateSite(c *fiber.Ctx) error {
	var req dto.CreateSiteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	identity, err := getIdentity(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.ErrorResponse{Error: err.Error()})
	}
	siteID, err := s.handlers.CreateSite.Execute(&req, identity)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	resp := dto.CreateSiteResponse{
		SiteID: siteID,
	}

	return c.Status(fiber.StatusCreated).JSON(resp)
}

func (s *Server) UpdateSite(c *fiber.Ctx, id uint64) error {
	var req dto.UpdateSiteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	identity, err := getIdentity(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.ErrorResponse{Error: err.Error()})
	}
	updatedSiteID, err := s.handlers.UpdateSite.Execute(id, &req, identity)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	resp := dto.UpdateSiteResponse{
		SiteID: updatedSiteID,
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (s *Server) EnrichContent(c *fiber.Ctx) error {
	var req dto.EnrichContentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	enrichContent, err := s.handlers.EnrichContent.Execute(&req)
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

func (s *Server) GetSite(c *fiber.Ctx, id uint64) error {

	identity, err := getIdentity(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.ErrorResponse{Error: err.Error()})
	}
	resp, err := s.handlers.GetSite.Query(id, identity)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (s *Server) GetToken(c *fiber.Ctx) error {
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

func (s *Server) CreatePayment(c *fiber.Ctx) error {
	var req dto.CreatePaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	identity, err := getIdentity(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.ErrorResponse{Error: err.Error()})
	}
	clientSecret, err := s.handlers.Payment.CreatePayment(&req, identity)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	resp := dto.CreatePaymentResponse{
		ClientSecret: clientSecret,
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (s *Server) GetPaymentStatus(c *fiber.Ctx, id string) error {

	paymentInfo, err := s.handlers.Payment.GetPaymentInfo(id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(paymentInfo)
}

func (s *Server) HandleEvent(c *fiber.Ctx) error {

	slog.Info("ARRIVED")
	signatureHeader := c.Get("Stripe-Signature")

	err := s.handlers.Payment.Webhook(c.Body(), signatureHeader)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.SendStatus(fiber.StatusOK)
}

func getToken(c *fiber.Ctx) string {
	head := c.Get("Authorization")
	parts := strings.SplitN(head, "Bearer: ", 2)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

func getIdentity(c *fiber.Ctx) (*auth.Identity, error) {
	return auth.IdentityProvider{}.GetIdentity(getToken(c))
}
