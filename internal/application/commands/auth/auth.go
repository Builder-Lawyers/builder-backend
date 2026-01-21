package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/consts"
	"github.com/Builder-Lawyers/builder-backend/internal/application/dto"
	"github.com/Builder-Lawyers/builder-backend/internal/application/events"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/auth"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db/repo"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/mail"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/Builder-Lawyers/builder-backend/pkg/interfaces"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Auth struct {
	uowFactory *dbs.UOWFactory
	cfg        auth.OIDCConfig
	cognito    *cognitoidentityprovider.Client
}

func NewAuth(uowFactory *dbs.UOWFactory, oidcCfg auth.OIDCConfig, cognito *cognitoidentityprovider.Client) *Auth {
	return &Auth{
		uowFactory: uowFactory,
		cfg:        oidcCfg,
		cognito:    cognito,
	}
}

func (c *Auth) CreateSession(ctx context.Context, req dto.CreateSession) (string, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*2)
	jwks, err := keyfunc.NewDefaultCtx(timeoutCtx, []string{c.cfg.IssuerURL})
	cancel()
	if err != nil {
		return "", fmt.Errorf("failed to get JWKS: %v", err)
	}

	accessClaims := &jwt.RegisteredClaims{}
	_, err = jwt.ParseWithClaims(req.AccessToken, accessClaims, jwks.Keyfunc, jwt.WithLeeway(10*time.Second))
	if err != nil {
		return "", fmt.Errorf("failed to parse JWT: %v", err)
	}

	idToken, _, err := new(jwt.Parser).ParseUnverified(req.IdToken, jwt.MapClaims{})
	if err != nil {
		return "", err
	}

	claims := idToken.Claims.(jwt.MapClaims)

	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return "", err
	}
	defer uow.Finalize(&err)

	var userID uuid.UUID
	err = tx.QueryRow(ctx, "SELECT id FROM builder.users WHERE email = $1",
		claims["email"].(string),
	).Scan(&userID)
	if err != nil {
		return "", fmt.Errorf("error getting user by email, %v", err)
	}

	session := db.Session{
		ID:           uuid.New(),
		UserID:       userID,
		RefreshToken: req.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Hour * time.Duration(c.cfg.SessionLifetimeHours)),
	}

	_, err = tx.Exec(ctx, "INSERT INTO builder.sessions(id, user_id, refresh_token, expires_at) VALUES ($1,$2,$3,$4)",
		session.ID, session.UserID, session.RefreshToken, session.ExpiresAt)
	if err != nil {
		return "", fmt.Errorf("error creating a session, %v", err)
	}

	return session.ID.String(), nil
}

func (c *Auth) CreateConfirmationCode(ctx context.Context, req *dto.CreateConfirmation) error {
	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return err
	}
	defer uow.Finalize(&err)

	code := uuid.New()
	expiresAt := time.Now().Add(time.Minute * time.Duration(c.cfg.ConfirmationExpirationMins))

	_, err = tx.Exec(ctx, "INSERT INTO builder.confirmation_codes(code, sub_id, email, expires_at) VALUES ($1,$2,$3,$4)",
		code, req.UserID, req.Email, expiresAt)
	if err != nil {
		return fmt.Errorf("err generating a confirmation code, %v", err)
	}

	newUserID := uuid.New()
	_, err = tx.Exec(ctx, "INSERT INTO builder.users(id, status, email, created_at) VALUES ($1,$2,$3,$4)",
		newUserID, consts.UserStatusNotConfirmed, req.Email, time.Now())
	if err != nil {
		return fmt.Errorf("err creating user, %v", err)
	}

	_, err = tx.Exec(ctx, "INSERT INTO builder.user_identities(id, provider, sub) VALUES($1,$2,$3)", newUserID, "Cognito", req.UserID)
	if err != nil {
		return fmt.Errorf("err creating user cognito identity, %v", err)
	}

	registrationConfirmData := mail.RegistrationConfirmData{
		Year:        strconv.Itoa(time.Now().Year()),
		RedirectURL: fmt.Sprintf("%v/%v", c.cfg.RedirectURL, code),
	}

	sendMail := events.SendMail{
		UserID:  newUserID.String(),
		Subject: registrationConfirmData.GetSubject(),
		Data:    registrationConfirmData,
	}

	eventRepo := repo.NewEventRepo(tx)
	err = eventRepo.InsertEvent(ctx, sendMail)
	if err != nil {
		return err
	}

	return nil
}

func (c *Auth) VerifyCode(ctx context.Context, req *dto.VerifyCode) (*dto.SessionInfo, string, error) {
	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return nil, "", err
	}
	defer uow.Finalize(&err)

	var status sql.NullString
	err = tx.QueryRow(ctx, "SELECT u.status FROM builder.confirmation_codes c "+
		"LEFT JOIN builder.users u "+
		"ON c.sub_id::uuid = u.id "+
		"WHERE c.code = $1", req.Code,
	).Scan(&status)
	if err != nil {
		return nil, "", fmt.Errorf("err getting confirmation code, %v", err)
	}

	if status.Valid && consts.UserStatus(status.String) == consts.UserConfirmed {
		return nil, "", fmt.Errorf("code is already used to confirm user")
	}

	var subID uuid.UUID
	var email string
	var expiresAt time.Time
	err = tx.QueryRow(ctx, "SELECT sub_id, email, expires_at FROM builder.confirmation_codes WHERE code = $1", req.Code).Scan(&subID, &email, &expiresAt)
	if err != nil {
		return nil, "", fmt.Errorf("err getting confirmation code, %v", err)
	}

	if expiresAt.Before(time.Now()) {
		// TODO: clear user from db and cognito so he can re-register
		return nil, "", fmt.Errorf("code is expired")
	}

	input := &cognitoidentityprovider.AdminConfirmSignUpInput{
		UserPoolId: aws.String(c.cfg.UserPoolID),
		Username:   aws.String(email),
	}

	_, err = c.cognito.AdminConfirmSignUp(ctx, input)
	if err != nil {
		return nil, "", fmt.Errorf("failed to confirm user: %v", err)
	}

	var userID uuid.UUID
	err = tx.QueryRow(ctx, "SELECT id FROM builder.user_identities WHERE provider = $1 AND sub = $2", "Cognito", subID).Scan(&userID)
	if err != nil {
		return nil, "", fmt.Errorf("err getting user from cognito sub, %v", err)
	}

	_, err = tx.Exec(ctx, "UPDATE builder.users SET status = $1 WHERE id = $2", consts.UserConfirmed, userID)
	if err != nil {
		return nil, "", fmt.Errorf("err updating user status, %v", err)
	}

	_, err = tx.Exec(ctx, "DELETE FROM builder.confirmation_codes WHERE code = $1", req.Code)
	if err != nil {
		return nil, "", fmt.Errorf("err deleting confirmation code")
	}

	registrationSuccessData := mail.RegistrationSuccessData{
		Year: strconv.Itoa(time.Now().Year()),
	}

	registrationSuccessMail := events.SendMail{
		UserID:  userID.String(),
		Subject: registrationSuccessData.GetSubject(),
		Data:    registrationSuccessData,
	}

	eventRepo := repo.NewEventRepo(tx)
	err = eventRepo.InsertEvent(ctx, registrationSuccessMail)
	if err != nil {
		return nil, "", err
	}

	session := db.Session{
		ID:           uuid.New(),
		UserID:       userID,
		RefreshToken: uuid.NewString(),
		ExpiresAt:    time.Now().Add(time.Hour * time.Duration(c.cfg.SessionLifetimeHours)),
	}

	_, err = tx.Exec(ctx, "INSERT INTO builder.sessions(id, user_id, refresh_token, expires_at) VALUES ($1,$2,$3,$4)",
		session.ID, session.UserID, session.RefreshToken, session.ExpiresAt)
	if err != nil {
		return nil, "", fmt.Errorf("error creating a session, %v", err)
	}

	sessionInfo, err := c.getSession(ctx, tx, session.ID)
	if err != nil {
		return nil, "", err
	}

	return sessionInfo, session.ID.String(), nil
}

func (c *Auth) VerifyOauth(ctx context.Context, req *dto.VerifyOauthToken) (*dto.SessionInfo, string, error) {

	var userID uuid.UUID
	claims, err := c.verifyGoogleIDToken(req.IdToken)
	if err != nil {
		return nil, "", err
	}

	email := claims["email"].(string)
	providerSub := claims["sub"].(string)

	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return nil, "", err
	}

	// check if user from this provider is already registered
	var existingUser sql.NullString
	err = tx.QueryRow(ctx, "SELECT id FROM builder.user_identities WHERE provider = $1 AND sub = $2",
		req.Provider, providerSub).Scan(&existingUser)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, "", fmt.Errorf("err checking if user already registered, %v", err)
		}
	}
	// user already registered with this provider -> create a new session or reuse existing
	if existingUser.Valid {
		_ = uow.Rollback()
		return c.createSessionIfNotExists(ctx, uuid.MustParse(existingUser.String))
	}
	defer uow.Finalize(&err)

	var existingUserAnotherProvider sql.NullString
	err = tx.QueryRow(ctx, "SELECT id FROM builder.users WHERE email = $1", email).Scan(&existingUserAnotherProvider)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, "", fmt.Errorf("err checking other identities, %v", err)
		}
	}

	// user already registered but with another provider -> create new user_identity
	if existingUserAnotherProvider.Valid {
		userID = uuid.MustParse(existingUserAnotherProvider.String)
		_, err = tx.Exec(ctx, "INSERT INTO builder.user_identities(id, provider, sub) VALUES($1,$2,$3)",
			userID, req.Provider, providerSub)
		if err != nil {
			return nil, "", fmt.Errorf("err creating new identity for existing user, %v", err)
		}
	} else {
		// user is new -> create
		firstName := ""
		secondName := ""
		if v, ok := claims["name"]; ok {
			nameParts := strings.Split(v.(string), " ")
			firstName = nameParts[0]
			if len(nameParts) == 2 {
				secondName = nameParts[1]
			}
		}
		userID = uuid.New()
		_, err = tx.Exec(ctx, "INSERT INTO builder.users(id, first_name, second_name, status, email, created_at) VALUES($1,$2,$3,$4,$5,$6)",
			userID, firstName, secondName, consts.UserConfirmed, email, time.Now())
		if err != nil {
			return nil, "", fmt.Errorf("err inserting user, %v", err)
		}

		_, err = tx.Exec(ctx, "INSERT INTO builder.user_identities(id, provider, sub) VALUES($1,$2,$3)", userID, req.Provider, providerSub)
		if err != nil {
			return nil, "", fmt.Errorf("err mapping user to identity, %v", err)
		}

		registrationSuccessData := mail.RegistrationSuccessData{
			Year: strconv.Itoa(time.Now().Year()),
		}

		registrationSuccessMail := events.SendMail{
			UserID:  userID.String(),
			Subject: registrationSuccessData.GetSubject(),
			Data:    registrationSuccessData,
		}

		eventRepo := repo.NewEventRepo(tx)
		err = eventRepo.InsertEvent(ctx, registrationSuccessMail)
		if err != nil {
			return nil, "", err
		}
	}

	session := db.Session{
		ID:           uuid.New(),
		UserID:       userID,
		RefreshToken: uuid.NewString(),
		ExpiresAt:    time.Now().Add(time.Hour * time.Duration(c.cfg.SessionLifetimeHours)),
	}

	_, err = tx.Exec(ctx, "INSERT INTO builder.sessions(id, user_id, refresh_token, expires_at) VALUES ($1,$2,$3,$4)",
		session.ID, session.UserID, session.RefreshToken, session.ExpiresAt)
	if err != nil {
		return nil, "", fmt.Errorf("err creating oauth2 based session, %v", err)
	}

	return &dto.SessionInfo{
		UserID: userID,
		Email:  email,
	}, session.ID.String(), nil
}

func (c *Auth) GetSession(ctx context.Context, id uuid.UUID) (*dto.SessionInfo, error) {
	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return nil, err
	}
	defer uow.Finalize(&err)

	// TODO: retrieve from cache
	return c.getSession(ctx, tx, id)
}

func (c *Auth) DeleteUser(ctx context.Context, req *dto.DeleteUserRequest) error {
	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return err
	}
	defer uow.Finalize(&err)

	var errDB error
	var userID uuid.UUID

	err = tx.QueryRow(ctx, "SELECT id FROM builder.users WHERE email = $1", req.Email).Scan(&userID)
	if err != nil {
		errDB = errors.Join(errDB, fmt.Errorf("err getting user by email %w", err))
	}

	identitiesToDelete, err := c.getUserIdentitiesToDelete(ctx, uow, userID)
	if err != nil {
		errDB = errors.Join(errDB, err)
	}

	if len(identitiesToDelete) > 0 {
		for _, identity := range identitiesToDelete {
			_, err = c.cognito.AdminDeleteUser(ctx, &cognitoidentityprovider.AdminDeleteUserInput{
				UserPoolId: &c.cfg.UserPoolID,
				Username:   aws.String(identity),
			})
			if err != nil {
				return fmt.Errorf("err deleting user from cognito: %v", err)
			}
		}

		_, err = tx.Exec(ctx, "DELETE FROM builder.user_identities WHERE id = $1", userID)
		if err != nil {
			return fmt.Errorf("err deleting user identities from db %v", err)
		}
	} else {
		_, err = c.cognito.AdminDeleteUser(ctx, &cognitoidentityprovider.AdminDeleteUserInput{
			UserPoolId: &c.cfg.UserPoolID,
			Username:   aws.String(req.Email),
		})
		if err != nil {
			return fmt.Errorf("err deleting user from cognito by email: %v", err)
		}
	}

	return nil
}

func (c *Auth) getUserIdentitiesToDelete(ctx context.Context, uow interfaces.UoW, userID uuid.UUID) ([]string, error) {
	var errDb error
	tx := uow.GetTx()

	_, err := tx.Exec(ctx, "DELETE FROM builder.users WHERE id = $1", userID)
	if err != nil {
		errDb = errors.Join(errDb, fmt.Errorf("err deleting user from db %w", err))
	}

	rows, err := tx.Query(ctx, "SELECT provider, sub FROM builder.user_identities WHERE id = $1", userID)
	if err != nil {
		errDb = errors.Join(errDb, fmt.Errorf("err getting user identities %w", err))
	}
	defer rows.Close()

	var identitiesToDelete []string
	for rows.Next() {
		var provider string
		var identity string
		if err := rows.Scan(&provider, &identity); err != nil {
			errDb = errors.Join(errDb, fmt.Errorf("err getting user identity %w", err))
		}
		if provider == "Cognito" {
			identitiesToDelete = append(identitiesToDelete, identity)
		}
	}
	if err := rows.Err(); err != nil {
		errDb = errors.Join(errDb, fmt.Errorf("err closing rows %w", err))
	}

	return identitiesToDelete, nil
}

func (c *Auth) GetIdentity(ctx context.Context, id uuid.UUID) (*auth.Identity, error) {
	if c.cfg.Mode == "TEST" {
		return &auth.Identity{
			UserID: c.cfg.TestUser,
		}, nil
	}
	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return nil, err
	}
	defer uow.Finalize(&err)

	// TODO: retrieve from cache
	var identity auth.Identity
	err = tx.QueryRow(ctx, "SELECT user_id FROM builder.sessions WHERE id = $1", id).Scan(&identity.UserID)
	if err != nil {
		return nil, fmt.Errorf("error getting session, %v", err)
	}

	return &identity, nil
}

func (c *Auth) ParseCookie(ctx context.Context, cookie string) (uuid.UUID, error) {
	if c.cfg.Mode == "TEST" {
		return uuid.New(), nil
	}
	if cookie == "" {
		return uuid.UUID{}, fmt.Errorf("session Cookie is absent")
	}
	sessionID, err := uuid.Parse(cookie)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("error parsing id cookie %v", err)
	}
	return sessionID, nil
}

func (c *Auth) getSession(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (*dto.SessionInfo, error) {
	var session dto.SessionInfo
	var siteID sql.NullInt64
	err := tx.QueryRow(ctx,
		"SELECT ss.user_id, u.email, s.id FROM builder.sessions ss "+
			"JOIN builder.users u "+
			"ON ss.user_id = u.id "+
			"LEFT JOIN builder.sites s "+
			"ON u.id = s.creator_id "+
			"WHERE ss.id = $1 LIMIT 1", userID,
	).Scan(&session.UserID, &session.Email, &siteID)
	if err != nil {
		return nil, fmt.Errorf("error getting session, %v", err)
	}

	if siteID.Valid {
		session.UserSite = &dto.UserSite{
			SiteID: uint64(siteID.Int64),
		}
	}

	return &session, nil
}

func (c *Auth) verifyGoogleIDToken(idToken string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

	jwks, err := keyfunc.NewDefaultCtx(ctx, []string{c.cfg.GoogleIssuerURL})
	cancel()
	if err != nil {
		return nil, fmt.Errorf("fetch jwks: %v", err)
	}

	claims := jwt.MapClaims{}
	_, err = jwt.ParseWithClaims(idToken, claims, jwks.Keyfunc, jwt.WithLeeway(10*time.Second))
	if err != nil {
		return nil, fmt.Errorf("parse token: %v", err)
	}

	return claims, nil
}

func (c *Auth) createSessionIfNotExists(ctx context.Context, userID uuid.UUID) (*dto.SessionInfo, string, error) {
	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return nil, "", err
	}
	defer uow.Finalize(&err)

	var sessionID uuid.UUID
	var existingSession sql.NullString
	err = tx.QueryRow(ctx, "SELECT id FROM builder.sessions WHERE user_id = $1 AND expires_at < $2",
		userID, time.Now()).Scan(&existingSession)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, "", fmt.Errorf("error getting user by email, %v", err)
		}
	}

	if !existingSession.Valid {
		session := db.Session{
			ID:           uuid.New(),
			UserID:       userID,
			RefreshToken: uuid.NewString(),
			ExpiresAt:    time.Now().Add(time.Hour * time.Duration(c.cfg.SessionLifetimeHours)),
		}
		_, err = tx.Exec(ctx, "INSERT INTO builder.sessions(id, user_id, refresh_token, expires_at) VALUES ($1,$2,$3,$4)",
			session.ID, session.UserID, session.RefreshToken, session.ExpiresAt)
		if err != nil {
			return nil, "", fmt.Errorf("err creating a session, %v", err)
		}
		sessionID = session.ID
	}

	var siteID sql.NullInt64
	var email string
	err = tx.QueryRow(ctx, "SELECT s.id, u.email FROM builder.users u "+
		"LEFT JOIN builder.sites s ON u.id = s.creator_id "+
		"WHERE u.id = $1 LIMIT 1", userID).Scan(&siteID, &email)
	if err != nil {
		return nil, "", fmt.Errorf("err getting existing user info, %v", err)
	}

	sessionInfo := &dto.SessionInfo{
		UserID: userID,
		Email:  email,
	}

	if siteID.Valid {
		sessionInfo.UserSite = &dto.UserSite{
			SiteID: uint64(siteID.Int64),
		}
	}

	return sessionInfo, sessionID.String(), nil
}

func createUserFromClaims(claims jwt.MapClaims) (*db.User, error) {
	id, err := uuid.Parse(claims["sub"].(string))
	if err != nil {
		return nil, err
	}
	return &db.User{
		ID:         id,
		FirstName:  claims["given_name"].(string),
		SecondName: claims["family_name"].(string),
		Email:      claims["email"].(string),
		CreatedAt:  time.Now(),
	}, nil
}
