package certs

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/acm/types"
)

type ACMCertificates struct {
	client *acm.Client
}

func NewACMCertificates(cfg aws.Config) *ACMCertificates {
	return &ACMCertificates{client: acm.NewFromConfig(cfg, func(o *acm.Options) {
		o.Region = "us-east-1" // region must be us-east-1 for CloudFront certificates
	})}
}

func (a *ACMCertificates) GetARN(ctx context.Context, arn string) (string, error) {
	res, err := a.client.GetCertificate(ctx, &acm.GetCertificateInput{CertificateArn: aws.String(arn)})
	if err != nil {
		return "", err
	}
	return aws.ToString(res.Certificate), nil
}

func (a *ACMCertificates) CreateCertificate(ctx context.Context, domain string) (string, error) {
	res, err := a.client.RequestCertificate(ctx, &acm.RequestCertificateInput{
		DomainName:       aws.String(domain),
		ValidationMethod: types.ValidationMethodDns,
	})
	if err != nil {
		return "", err
	}

	return aws.ToString(res.CertificateArn), nil
}
