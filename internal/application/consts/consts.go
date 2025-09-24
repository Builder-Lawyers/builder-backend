package consts

type SiteStatus string

const SiteStatusInCreation SiteStatus = "InCreation"
const SiteStatusAwaitingProvision SiteStatus = "AwaitingProvision"
const SiteStatusCreated SiteStatus = "Created"
const SiteStatusDeactivated SiteStatus = "Deactivated"
const SiteStatusAwaitingDeactivation SiteStatus = "AwaitingDeactivation"
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
