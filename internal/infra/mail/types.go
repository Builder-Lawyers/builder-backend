package mail

type MailType string

const (
	SiteCreated         MailType = "SiteCreated"
	SiteDeactivated     MailType = "SiteDeactivated"
	RegistrationSuccess MailType = "RegistrationSuccess"
	FreeTrialEnds       MailType = "FreeTrialEnds"
)

type MailData interface {
	GetMailType() MailType
	GetSubject() string
}

type SiteCreatedData struct {
	CustomerFirstName  string
	CustomerSecondName string
	SiteURL            string
	Year               string
}

func (s SiteCreatedData) GetMailType() MailType {
	return SiteCreated
}

func (s SiteCreatedData) GetSubject() string {
	return "Your site was successfully created!"
}

type FreeTrialEndsData struct {
	DaysUntilEnd       int
	PaymentURL         string
	Year               string
	CustomerFirstName  string
	CustomerSecondName string
}

func (s FreeTrialEndsData) GetMailType() MailType {
	return FreeTrialEnds
}

func (s FreeTrialEndsData) GetSubject() string {
	return "Your free trial period is about to end"
}

type SiteDeactivatedData struct {
	Year               string
	SiteURL            string
	Reason             string
	CustomerFirstName  string
	CustomerSecondName string
}

func (s SiteDeactivatedData) GetMailType() MailType {
	return SiteDeactivated
}

func (s SiteDeactivatedData) GetSubject() string {
	return "Your site was deactivated"
}
