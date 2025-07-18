package db

import (
	"github.com/Builder-Lawyers/builder-backend/builder/internal/domain/consts"
	"github.com/Builder-Lawyers/builder-backend/builder/internal/domain/entity"
)

func MapSiteModelToEntity(site Site, user User) entity.Site {
	return entity.Site{
		ID:         site.ID,
		TemplateID: site.TemplateID,
		Status:     consts.Status(site.Status),
		Creator:    MapUserModelToEntity(user),
	}
}

func MapUserModelToEntity(user User) entity.User {
	return entity.User{
		ID:           user.ID,
		Name:         user.FirstName,
		Surname:      user.SecondName,
		Email:        user.Email,
		RegisteredAt: user.CreatedAt,
	}
}
