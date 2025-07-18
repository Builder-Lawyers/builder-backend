package entity

import (
	"builder/internal/domain/consts"
)

type Site struct {
	ID         uint64
	TemplateID uint8
	Status     consts.Status
	Creator    User
	// is in free tier??
	// if a user deletes site at the middle of the month, do we ask a half of month's worth or nothing?
}

func NewSite(templateID uint8, creator User) *Site {
	return &Site{
		TemplateID: templateID,
		Status:     consts.InCreation,
		Creator:    creator,
	}
}

func (e Site) UpdateState(newStatus consts.Status) {
	e.Status = newStatus
}
