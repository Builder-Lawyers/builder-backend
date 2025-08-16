package consts

type OutboxStatus int

const (
	NotProcessed OutboxStatus = iota
	Processed
)

type ProvisionType int

const (
	DefaultDomain ProvisionType = iota
	SeparateDomain
	BringYourDomain
)

type ProvisionStatus string

const (
	ProvisionStatusInProcess   ProvisionStatus = "IN_PROCESS"
	ProvisionStatusProvisioned ProvisionStatus = "PROVISIONED"
	ProvisionStatusInError     ProvisionStatus = "IN_ERROR"
)
