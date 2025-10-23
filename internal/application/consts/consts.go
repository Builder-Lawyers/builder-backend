package consts

type SiteStatus string

const (
	SiteStatusInCreation           SiteStatus = "InCreation"
	SiteStatusAwaitingProvision    SiteStatus = "AwaitingProvision"
	SiteStatusCreated              SiteStatus = "Created"
	SiteStatusDeactivated          SiteStatus = "Deactivated"
	SiteStatusAwaitingDeactivation SiteStatus = "AwaitingDeactivation"
	SiteStatusDeleted              SiteStatus = "Deleted"
)

type UserStatus string

const (
	UserStatusNotConfirmed UserStatus = "NotConfirmed"
	UserConfirmed          UserStatus = "Confirmed"
)

type OutboxStatus int

const (
	NotProcessed OutboxStatus = iota
	Processed
)

type ProvisionType string

const (
	DefaultDomain   ProvisionType = "DefaultDomain"
	SeparateDomain  ProvisionType = "SeparateDomain"
	BringYourDomain ProvisionType = "BringYourDomain"
)

type ProvisionStatus string

const (
	ProvisionStatusInProcess   ProvisionStatus = "IN_PROCESS"
	ProvisionStatusProvisioned ProvisionStatus = "PROVISIONED"
	ProvisionStatusInError     ProvisionStatus = "IN_ERROR"
	ProvisionStatusDeactivated ProvisionStatus = "DEACTIVATED"
)
