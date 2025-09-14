package commands

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/dto"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/auth"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/coreos/go-oidc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

type Auth struct {
	cache       map[string]string
	uowFactory  *dbs.UOWFactory
	cfg         *auth.OIDCConfig
	oauthClient *oauth2.Config
}

func NewAuth(uowFactory *dbs.UOWFactory, cfg *auth.OIDCConfig) *Auth {
	provider, err := oidc.NewProvider(context.Background(), cfg.IssuerURL)
	if err != nil {
		log.Panicln("Failed to create OIDC provider:", err)
	}
	return &Auth{
		cache:      make(map[string]string),
		uowFactory: uowFactory,
		cfg:        cfg,
		oauthClient: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "profile", "openid", "email"},
		},
	}
}

func (c *Auth) CreateSession(req dto.CreateSession) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	jwks, err := keyfunc.NewDefaultCtx(ctx, []string{c.cfg.IssuerURL + "/.well-known/jwks.json"})
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
	err = tx.QueryRow(context.Background(), "SELECT id FROM builder.users WHERE email = $1",
		claims["email"].(string),
	).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			newUser, err := createUserFromClaims(claims)
			if err != nil {
				return "", fmt.Errorf("err creating new user, %v", err)
			}
			userID = newUser.ID
			_, err = tx.Exec(context.Background(), "INSERT INTO builder.users(id, first_name, second_name, email, created_at) VALUES ($1,$2,$3,$4,$5)",
				newUser.ID, newUser.FirstName, newUser.SecondName, newUser.Email, newUser.CreatedAt,
			)
			if err != nil {
				return "", fmt.Errorf("err inserting user, %v", err)
			}
		}
		return "", fmt.Errorf("error getting user by email, %v", err)
	}

	session := db.Session{
		ID:           uuid.New(),
		UserID:       userID,
		RefreshToken: req.RefreshToken,
		IssuedAt:     time.Now(),
	}

	_, err = tx.Exec(context.Background(), "INSERT INTO builder.sessions(id, user_id, refresh_token, issued_at) VALUES ($1,$2,$3,$4)",
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

func (c *Auth) GetIdentity(id uuid.UUID) (*auth.Identity, error) {
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
	err = tx.QueryRow(context.Background(), "SELECT user_id FROM builder.sessions WHERE id = $1", id).Scan(&identity.UserID)
	if err != nil {
		return nil, fmt.Errorf("error getting session, %v", err)
	}

	return &identity, nil
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
