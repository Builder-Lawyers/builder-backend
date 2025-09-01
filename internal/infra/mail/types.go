package mail

type MailType string

const (
	SiteCreated         MailType = "SiteCreated"
	RegistrationSuccess MailType = "RegistrationSuccess"
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
