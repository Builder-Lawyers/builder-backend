package consts

type OutboxStatus int

const (
	NotProcessed OutboxStatus = iota
	Processed
)
