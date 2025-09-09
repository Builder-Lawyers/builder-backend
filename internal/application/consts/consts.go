package consts

type SiteStatus string

const SiteStatusInCreation SiteStatus = "SiteStatusInCreation"
const SiteStatusAwaitingProvision SiteStatus = "SiteStatusAwaitingProvision"
const SiteStatusCreated SiteStatus = "SiteStatusCreated"
const SiteStatusDeactivated SiteStatus = "Deactivated"
const SiteStatusDeleted SiteStatus = "Deleted"

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
