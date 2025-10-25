package rest

import (
	"fmt"
	"strings"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application"
	"github.com/Builder-Lawyers/builder-backend/internal/application/dto"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/auth"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
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

	identity, err := s.getIdentity(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.ErrorResponse{Error: err.Error()})
	}
	siteID, err := s.handlers.CreateSite.Execute(c.UserContext(), &req, identity)
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

	identity, err := s.getIdentity(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.ErrorResponse{Error: err.Error()})
	}
	updatedSiteID, err := s.handlers.UpdateSite.Execute(c.UserContext(), id, &req, identity)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	resp := dto.UpdateSiteResponse{
		SiteID: updatedSiteID,
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (s *Server) DeleteSite(c *fiber.Ctx, id uint64) error {

	identity, err := s.getIdentity(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	err = s.handlers.DeleteSite.Execute(c.UserContext(), id, identity)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) CreateTemplate(c *fiber.Ctx) error {
	var req dto.CreateTemplateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	templateID, err := s.handlers.CreateTemplate.Execute(c.UserContext(), &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	resp := dto.CreateTemplateResponse{
		Id: templateID,
	}

	return c.Status(fiber.StatusNoContent).JSON(resp)
}

func (s *Server) EnrichContent(c *fiber.Ctx) error {
	var req dto.EnrichContentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	enrichContent, err := s.handlers.EnrichContent.Execute(c.UserContext(), &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	resp := dto.EnrichContentResponse{
		Enriched: enrichContent,
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (s *Server) FileUpload(c *fiber.Ctx) error {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	resp, err := s.handlers.UploadFile.Execute(c.UserContext(), fileHeader)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(resp)
}

func (s *Server) GetTemplate(c *fiber.Ctx, id uint8) error {

	templateInfo, err := s.handlers.GetTemplate.Query(c.UserContext(), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(templateInfo)
}

func (s *Server) CheckDomain(c *fiber.Ctx, domain string) error {

	available, err := s.handlers.CheckDomain.Query(c.UserContext(), domain)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}
	resp := dto.DomainAvailability{
		Available: available,
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (s *Server) GetSite(c *fiber.Ctx, id uint64) error {

	identity, err := s.getIdentity(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.ErrorResponse{Error: err.Error()})
	}
	resp, err := s.handlers.GetSite.Query(c.UserContext(), id, identity)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (s *Server) CreateSession(c *fiber.Ctx) error {
	var req dto.CreateSession
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	sessionID, err := s.handlers.Auth.CreateSession(c.UserContext(), req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	c.Cookie(&fiber.Cookie{
		Name:     "ID",
		Value:    sessionID,
		Expires:  time.Now().Add(24 * time.Hour), // 1 day
		HTTPOnly: true,                           // prevent JS access
		Secure:   false,                          // TODO: set to true to send only over HTTPS
		SameSite: "Strict",                       // protect against CSRF
	})

	return c.SendStatus(fiber.StatusOK)
}

func (s *Server) CreateConfirmation(c *fiber.Ctx) error {
	var req dto.CreateConfirmation
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	err := s.handlers.Auth.CreateConfirmationCode(c.UserContext(), &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.SendStatus(fiber.StatusCreated)
}

func (s *Server) VerifyUser(c *fiber.Ctx) error {
	var req dto.VerifyCode
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	verifiedUser, err := s.handlers.Auth.VerifyCode(c.UserContext(), &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(verifiedUser)
}

func (s *Server) VerifyOauthToken(c *fiber.Ctx) error {
	var req dto.VerifyOauthToken
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	verifiedUser, err := s.handlers.Auth.VerifyOauth(c.UserContext(), &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(verifiedUser)
}

func (s *Server) CreatePayment(c *fiber.Ctx) error {
	var req dto.CreatePaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	identity, err := s.getIdentity(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.ErrorResponse{Error: err.Error()})
	}
	clientSecret, err := s.handlers.Payment.CreatePayment(c.UserContext(), &req, identity)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	resp := dto.CreatePaymentResponse{
		ClientSecret: &clientSecret,
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (s *Server) GetPaymentStatus(c *fiber.Ctx, id string) error {

	paymentInfo, err := s.handlers.Payment.GetPaymentInfo(c.UserContext(), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(paymentInfo)
}

func (s *Server) HandleEvent(c *fiber.Ctx) error {

	signatureHeader := c.Get("Stripe-Signature")

	err := s.handlers.Payment.Webhook(c.UserContext(), c.Body(), signatureHeader)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.SendStatus(fiber.StatusOK)
}

func (s *Server) getIdentity(c *fiber.Ctx) (*auth.Identity, error) {
	session, err := getSessionID(c)
	if err != nil {
		return nil, err
	}
	identity, err := s.handlers.Auth.GetIdentity(c.UserContext(), session)
	if err != nil {
		return nil, err
	}

	return identity, nil
}

func getSessionID(c *fiber.Ctx) (uuid.UUID, error) {
	cookie := c.Cookies("ID", "")
	if cookie == "" {
		return uuid.UUID{}, fmt.Errorf("session Cookie is absent")
	}

	sessionID, err := uuid.Parse(cookie)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("error parsing id cookie %v", err)
	}

	return sessionID, nil
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
