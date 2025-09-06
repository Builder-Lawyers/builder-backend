package consts

type Status string

const InCreation Status = "InCreation"
const AwaitingProvision Status = "AwaitingProvision"
const Created Status = "Created"

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
)
