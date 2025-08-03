package certs

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
)

type ACMCertificates struct {
	client *acm.Client
}

func NewACMCertificates(cfg aws.Config) *ACMCertificates {
	return &ACMCertificates{client: acm.NewFromConfig(cfg)}
}

func (a *ACMCertificates) GetARN(id string) (string, error) {
	res, err := a.client.GetCertificate(context.Background(), &acm.GetCertificateInput{CertificateArn: aws.String(id)})
	if err != nil {
		return "", err
	}
	return *res.Certificate, nil
}
