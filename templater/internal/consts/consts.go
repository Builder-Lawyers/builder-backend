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

type ProvisionStatus int

const (
	InProcess ProvisionStatus = iota
	Provisioned
	InError
)
