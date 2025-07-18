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
