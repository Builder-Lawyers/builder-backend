package auth

import (
	"fmt"

	"github.com/golang-jwt/jwt/v4"
)

type IdentityProvider struct {
}

type Identity struct {
	UserID string
}

func (p IdentityProvider) GetIdentity(tokenString string) (*Identity, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("identity can't be retrieved, %v", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if ok && token.Valid {
		fmt.Println("Token is valid")
		fmt.Println("sub:", claims["sub"])
	}

	return &Identity{
		UserID: claims["sub"].(string),
	}, nil
}
