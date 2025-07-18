package templater

type ProvisionSiteReq struct {
	TemplateID uint8  `json:"templateID"`
	Fields     Fields `json:"fields"`
}

type Fields struct {
}
