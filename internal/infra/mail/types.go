package mail

type MailType string

const (
	SiteCreated         MailType = "SiteCreated"
	RegistrationSuccess MailType = "RegistrationSuccess"
	FreeTrialEnds       MailType = "FreeTrialEnds"
)

type MailData interface {
	GetMailType() MailType
}

type SiteCreatedData struct {
	Name    string
	Surname string
	SiteURL string
}

func (s SiteCreatedData) GetMailType() MailType {
	return SiteCreated
}

type FreeTrialEndsData struct {
	DaysUntilEnd int
	PaymentURL   string
}

func (s FreeTrialEndsData) GetMailType() MailType {
	return FreeTrialEnds
}
