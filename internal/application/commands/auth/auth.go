package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/consts"
	"github.com/Builder-Lawyers/builder-backend/internal/application/dto"
	"github.com/Builder-Lawyers/builder-backend/internal/application/events"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/auth"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db/repo"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/mail"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Auth struct {
	uowFactory *dbs.UOWFactory
	cfg        *auth.OIDCConfig
	cognito    *cognitoidentityprovider.Client
}

func NewAuth(uowFactory *dbs.UOWFactory, oidcCfg *auth.OIDCConfig, cfg aws.Config) *Auth {
	return &Auth{
		uowFactory: uowFactory,
		cfg:        oidcCfg,
		cognito: cognitoidentityprovider.NewFromConfig(cfg, func(o *cognitoidentityprovider.Options) {
			o.Region = "us-east-1"
		}),
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

	//claims, ok := token.Claims.(jwt.MapClaims)
	//if ok && token.Valid {
	//	fmt.Println("Token is valid")
	//	fmt.Println("sub:", claims["sub"])
	//} else {
	//	return "", fmt.Errorf("invalid token")
	//}

	//c.cache[claims["sub"].(string)] = req.AccessToken

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
	var userID uuid.UUID
	err = tx.QueryRow(ctx, "SELECT id FROM builder.users WHERE email = $1",
		claims["email"].(string),
	).Scan(&userID)
	if err != nil {
		//if errors.Is(err, sql.ErrNoRows) {
		//	newUser, err := createUserFromClaims(claims)
		//	if err != nil {
		//		return "", fmt.Errorf("err creating new user, %v", err)
		//	}
		//	userID = newUser.ID
		//	_, err = tx.Exec(ctx, "INSERT INTO builder.users(id, first_name, second_name, email, created_at) VALUES ($1,$2,$3,$4,$5)",
		//		newUser.ID, newUser.FirstName, newUser.SecondName, newUser.Email, newUser.CreatedAt,
		//	)
		//	if err != nil {
		//		return "", fmt.Errorf("err inserting user, %v", err)
		//	}
		//}
		return "", fmt.Errorf("error getting user by email, %v", err)
	}

	session := db.Session{
		ID:           uuid.New(),
		UserID:       userID,
		RefreshToken: req.RefreshToken,
		IssuedAt:     time.Now(),
	}

	_, err = tx.Exec(ctx, "INSERT INTO builder.sessions(id, user_id, refresh_token, issued_at) VALUES ($1,$2,$3,$4)",
		session.ID, session.UserID, session.RefreshToken, session.IssuedAt)
	if err != nil {
		return "", fmt.Errorf("error creating a session, %v", err)
	}

	err = uow.Commit()
	if err != nil {
		return "", fmt.Errorf("error commiting tx, %v", err)
	}

	return session.ID.String(), nil
}

func (c *Auth) CreateConfirmationCode(ctx context.Context, req *dto.CreateConfirmation) error {

	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return err
	}

	code := uuid.New()
	expiresAt := time.Now().Add(time.Minute * time.Duration(c.cfg.ConfirmationExpirationMins))

	_, err = tx.Exec(ctx, "INSERT INTO builder.confirmation_codes(code, user_id, email, expires_at) VALUES ($1,$2,$3,$4)",
		code, req.UserID, req.Email, expiresAt)
	if err != nil {
		return fmt.Errorf("err generating a confirmation code, %v", err)
	}

	_, err = tx.Exec(ctx, "INSERT INTO builder.users(id, status, email, created_at) VALUES ($1,$2,$3,$4)",
		req.UserID, consts.UserStatusNotConfirmed, req.Email, time.Now())
	if err != nil {
		return fmt.Errorf("err creating user, %v", err)
	}

	registrationConfirmData := mail.RegistrationConfirmData{
		Year:        strconv.Itoa(time.Now().Year()),
		RedirectURL: fmt.Sprintf("%v?code=%v", c.cfg.RedirectURL, code),
	}

	sendMail := events.SendMail{
		UserID:  req.UserID,
		Subject: registrationConfirmData.GetSubject(),
		Data:    registrationConfirmData,
	}

	eventRepo := repo.NewEventRepo(tx)
	err = eventRepo.InsertEvent(ctx, sendMail)
	if err != nil {
		return fmt.Errorf("error creating event, %v", err)
	}

	err = uow.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (c *Auth) VerifyCode(ctx context.Context, req *dto.VerifyCode) (*dto.VerifiedUser, error) {
	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return nil, err
	}

	var userID uuid.UUID
	var email string
	var expiresAt time.Time
	err = tx.QueryRow(ctx, "SELECT user_id, email, expires_at FROM builder.confirmation_codes WHERE code = $1", req.Code).Scan(&userID, &email, &expiresAt)
	if err != nil {
		return nil, fmt.Errorf("err getting confirmation code, %v", err)
	}

	if expiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("code is expired")
	}

	input := &cognitoidentityprovider.AdminConfirmSignUpInput{
		UserPoolId: aws.String(c.cfg.UserPoolID),
		Username:   aws.String(email), // TODO: or email?
	}

	_, err = c.cognito.AdminConfirmSignUp(context.Background(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to confirm user: %v", err)
	}

	_, err = tx.Exec(ctx, "UPDATE builder.users SET status = $1 WHERE id = $2", consts.UserConfirmed, userID)
	if err != nil {
		return nil, fmt.Errorf("err updating user status, %v", err)
	}

	// TODO: send registration success mail

	err = uow.Commit()
	if err != nil {
		return nil, err
	}

	return &dto.VerifiedUser{
		UserID: userID.String(),
	}, nil
}

func (c *Auth) VerifyOauth(ctx context.Context, req *dto.VerifyOauthToken) (*dto.OauthTokenVerified, error) {

	claims, err := c.verifyGoogleIDToken(req.IdToken)
	if err != nil {
		return nil, err
	}

	email := claims["email"].(string)
	sub := claims["sub"].(string)
	slog.Info(sub)

	var userID uuid.UUID
	adminCreateResponse, err := c.cognito.AdminCreateUser(ctx, &cognitoidentityprovider.AdminCreateUserInput{
		UserPoolId: &c.cfg.UserPoolID,
		Username:   &email,
		UserAttributes: []types.AttributeType{
			{Name: aws.String("email"), Value: aws.String(email)},
			{Name: aws.String("email_verified"), Value: aws.String("true")},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("admin create: %v", err)
	}
	for _, attribute := range adminCreateResponse.User.Attributes {
		if *attribute.Name == "sub" {
			userID = uuid.MustParse(*attribute.Value)
		}
	}

	//_, err = c.cognito.AdminConfirmSignUp(context.Background(), input)
	//if err != nil {
	//	return nil, fmt.Errorf("failed to confirm user: %v", err)
	//}

	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, "INSERT INTO builder.users(id, status, email, created_at)",
		userID, consts.UserConfirmed, email, time.Now())
	if err != nil {
		return nil, fmt.Errorf("err inserting user, %v", err)
	}

	session := db.Session{
		ID:           uuid.New(),
		UserID:       userID,
		RefreshToken: "",
		IssuedAt:     time.Now(),
	}

	_, err = tx.Exec(ctx, "INSERT INTO builder.sessions(id, user_id, refresh_token, issued_at) VALUES ($1,$2,$3,$4)",
		session.ID, session.UserID, session.RefreshToken, session.IssuedAt)

	err = uow.Commit()
	if err != nil {
		return nil, err
	}

	return &dto.OauthTokenVerified{
		UserID: userID.String(),
	}, nil
}

func (c *Auth) GetIdentity(ctx context.Context, id uuid.UUID) (*auth.Identity, error) {
	if c.cfg.Mode == "TEST" {
		return &auth.Identity{
			UserID: *c.cfg.TestUser,
		}, nil
	}
	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return nil, err
	}

	// TODO: retrieve from cache
	var identity auth.Identity
	err = tx.QueryRow(ctx, "SELECT user_id FROM builder.sessions WHERE id = $1", id).Scan(&identity.UserID)
	if err != nil {
		return nil, fmt.Errorf("error getting session, %v", err)
	}

	return &identity, nil
}

func (c *Auth) verifyGoogleIDToken(idToken string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

	jwks, err := keyfunc.NewDefaultCtx(ctx, []string{c.cfg.IssuerURL})
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
