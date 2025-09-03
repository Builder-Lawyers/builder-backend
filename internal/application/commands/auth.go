package commands

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/infra/auth"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/MicahParks/keyfunc"
	"github.com/coreos/go-oidc"
	"github.com/golang-jwt/jwt/v4"
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

func (c *Auth) Callback(code string) (string, error) {
	rawToken, err := c.oauthClient.Exchange(context.Background(), code)
	if err != nil {
		return "", err
	}
	tokenString := rawToken.AccessToken

	jwks, err := keyfunc.Get(c.cfg.IssuerURL+"/.well-known/jwks.json", keyfunc.Options{})
	if err != nil {
		return "", fmt.Errorf("failed to get JWKS: %v", err)
	}

	token, err := jwt.Parse(tokenString, jwks.Keyfunc)
	if err != nil {
		return "", fmt.Errorf("failed to parse JWT: %v", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if ok && token.Valid {
		fmt.Println("Token is valid")
		fmt.Println("sub:", claims["sub"])
	} else {
		return "", fmt.Errorf("invalid token")
	}

	// tests
	iat, _ := claims["iat"].(float64)
	exp, _ := claims["exp"].(float64)

	fmt.Println("Token claims:", claims)
	fmt.Printf("iat (issued at): %v (%s)\n", int64(iat), time.Unix(int64(iat), 0).UTC())
	fmt.Printf("exp (expires at): %v (%s)\n", int64(exp), time.Unix(int64(exp), 0).UTC())

	// Compare with system clock
	now := time.Now().UTC()
	fmt.Printf("System clock now: %v (%d)\n", now, now.Unix())

	diff := now.Unix() - int64(iat)
	fmt.Printf("Clock skew relative to iat: %d seconds\n", diff)
	// tests

	c.cache[claims["sub"].(string)] = tokenString

	idTokenRaw, ok := rawToken.Extra("id_token").(string)
	if !ok {
		return "", fmt.Errorf("no id_token field in oauth2 token response")
	}
	fmt.Println(idTokenRaw)
	idToken, _, err := new(jwt.Parser).ParseUnverified(idTokenRaw, jwt.MapClaims{})
	if err != nil {
		return "", err
	}

	claims = idToken.Claims.(jwt.MapClaims)

	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return "", err
	}
	var userID string
	err = tx.QueryRow(context.Background(), "SELECT id FROM builder.users WHERE email = $1",
		claims["email"].(string),
	).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			newUser, err := createUserFromClaims(claims)
			if err != nil {
				return "", err
			}
			_, err = tx.Exec(context.Background(), "INSERT INTO builder.users(id, first_name, second_name, email, created_at) VALUES ($1,$2,$3,$4,$5)",
				newUser.ID, newUser.FirstName, newUser.SecondName, newUser.Email, newUser.CreatedAt,
			)
			if err != nil {
				return "", err
			}
			return tokenString, nil
		}
		return "", err
	}
	return tokenString, nil
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
