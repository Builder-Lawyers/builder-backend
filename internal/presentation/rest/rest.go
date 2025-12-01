package rest

import (
	"fmt"
	"log/slog"
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
	queries  *application.Queries
	commands *application.Commands
}

func NewServer(queries *application.Queries, commands *application.Commands) *Server {
	return &Server{queries: queries, commands: commands}
}

func (s *Server) CreateSite(c *fiber.Ctx) error {
	var req dto.CreateSiteRequest
	var err error
	defer logError(&err, "CreateSite")
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	identity, err := s.getIdentity(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.ErrorResponse{Error: err.Error()})
	}
	siteID, err := s.commands.CreateSite.Execute(c.UserContext(), &req, identity)
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
	var err error
	defer logError(&err, "UpdateSite")
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	identity, err := s.getIdentity(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.ErrorResponse{Error: err.Error()})
	}
	updatedSiteID, err := s.commands.UpdateSite.Execute(c.UserContext(), id, &req, identity)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	resp := dto.UpdateSiteResponse{
		SiteID: updatedSiteID,
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (s *Server) DeleteSite(c *fiber.Ctx, id uint64) error {
	var err error
	defer logError(&err, "DeleteSite")
	identity, err := s.getIdentity(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	err = s.commands.DeleteSite.Execute(c.UserContext(), id, identity)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) CreateTemplate(c *fiber.Ctx) error {
	var err error
	defer logError(&err, "CreateTemplate")
	var req dto.CreateTemplateRequest
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	templateID, err := s.commands.CreateTemplate.Execute(c.UserContext(), &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	resp := dto.CreateTemplateResponse{
		Id: templateID,
	}

	return c.Status(fiber.StatusNoContent).JSON(resp)
}

func (s *Server) UpdateTemplates(c *fiber.Ctx) error {
	var err error
	defer logError(&err, "UpdateTemplates")
	var req dto.UpdateTemplatesRequest
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	err = s.commands.UpdateTemplate.Execute(c.UserContext(), &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.SendStatus(fiber.StatusOK)
}

func (s *Server) EnrichContent(c *fiber.Ctx) error {
	var req dto.EnrichContentRequest
	var err error
	defer logError(&err, "EnrichContent")
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	enrichContent, err := s.commands.EnrichContent.Execute(c.UserContext(), &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	resp := dto.EnrichContentResponse{
		Enriched: enrichContent,
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (s *Server) FileUpload(c *fiber.Ctx) error {
	var err error
	defer logError(&err, "FileUpload")
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	resp, err := s.commands.UploadFile.Execute(c.UserContext(), fileHeader)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(resp)
}

func (s *Server) GetTemplate(c *fiber.Ctx, id uint16) error {
	var err error
	defer logError(&err, "GetTemplate")
	templateInfo, err := s.queries.GetTemplate.Query(c.UserContext(), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(templateInfo)
}

func (s *Server) ListTemplates(c *fiber.Ctx) error {
	var err error
	defer logError(&err, "ListTemplates")
	var req dto.ListTemplatePaginator
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	resp, err := s.queries.GetTemplate.QueryList(c.UserContext(), &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (s *Server) CheckDomain(c *fiber.Ctx, domain string) error {
	var err error
	defer logError(&err, "CheckDomain")
	available, err := s.queries.CheckDomain.Query(c.UserContext(), domain)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}
	resp := dto.DomainAvailability{
		Available: available,
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (s *Server) GetSite(c *fiber.Ctx, id uint64) error {
	var err error
	defer logError(&err, "GetSite")
	identity, err := s.getIdentity(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.ErrorResponse{Error: err.Error()})
	}
	resp, err := s.queries.GetSite.Query(c.UserContext(), id, identity)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (s *Server) GetSession(c *fiber.Ctx) error {
	var err error
	defer logError(&err, "GetSession")
	sessionID, err := getSessionID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	identity, err := s.commands.Auth.GetSession(c.UserContext(), sessionID)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(&identity)
}

func (s *Server) CreateSession(c *fiber.Ctx) error {
	var req dto.CreateSession
	var err error
	defer logError(&err, "CreateSession")
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	sessionID, err := s.commands.Auth.CreateSession(c.UserContext(), req)
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
	var err error
	defer logError(&err, "CreateConfirmation")
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	err = s.commands.Auth.CreateConfirmationCode(c.UserContext(), &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.SendStatus(fiber.StatusCreated)
}

func (s *Server) VerifyUser(c *fiber.Ctx) error {
	var req dto.VerifyCode
	var err error
	defer logError(&err, "VerifyUser")
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	sessionInfo, sessionID, err := s.commands.Auth.VerifyCode(c.UserContext(), &req)
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

	return c.Status(fiber.StatusOK).JSON(sessionInfo)
}

func (s *Server) VerifyOauthToken(c *fiber.Ctx) error {
	var req dto.VerifyOauthToken
	var err error
	defer logError(&err, "VerifyOauthToken")
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	verifiedUser, sessionID, err := s.commands.Auth.VerifyOauth(c.UserContext(), &req)
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

	return c.Status(fiber.StatusCreated).JSON(verifiedUser)
}

func (s *Server) DeleteUser(c *fiber.Ctx) error {
	var req dto.DeleteUserRequest
	var err error
	defer logError(&err, "DeleteUser")
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	err = s.commands.Auth.DeleteUser(c.UserContext(), &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) ListPaymentPlans(c *fiber.Ctx) error {
	var err error
	defer logError(&err, "ListPaymentPlans")
	resp, err := s.commands.Payment.ListPaymentPlans(c.UserContext())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (s *Server) CreatePayment(c *fiber.Ctx) error {
	var err error
	defer logError(&err, "CreatePayment")
	var req dto.CreatePaymentRequest
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	identity, err := s.getIdentity(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.ErrorResponse{Error: err.Error()})
	}
	clientSecret, err := s.commands.Payment.CreatePayment(c.UserContext(), &req, identity)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	resp := dto.CreatePaymentResponse{
		ClientSecret: &clientSecret,
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (s *Server) GetPaymentStatus(c *fiber.Ctx, id string) error {
	var err error
	defer logError(&err, "GetPaymentStatus")
	paymentInfo, err := s.commands.Payment.GetPaymentInfo(c.UserContext(), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(paymentInfo)
}

func (s *Server) HandleEvent(c *fiber.Ctx) error {
	var err error
	defer logError(&err, "HandleEvent")
	signatureHeader := c.Get("Stripe-Signature")

	err = s.commands.Payment.Webhook(c.UserContext(), c.Body(), signatureHeader)
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
	identity, err := s.commands.Auth.GetIdentity(c.UserContext(), session)
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

func logError(err *error, endpoint string) {
	if *err != nil {
		slog.Error("server error", "endpoint", endpoint, "err", *err)
	}
}

func getToken(c *fiber.Ctx) string {
	head := c.Get("Authorization")
	parts := strings.SplitN(head, "Bearer: ", 2)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}
