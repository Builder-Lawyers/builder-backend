package dns

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	rTypes "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/aws-sdk-go-v2/service/route53domains"
	rdTypes "github.com/aws/aws-sdk-go-v2/service/route53domains/types"
)

type DNSProvisioner struct {
	domainContact *DomainContact
	client        *route53.Client
	domainClient  *route53domains.Client
	cfClient      *cloudfront.Client
}

func NewDNSProvisioner(awsConfig aws.Config, domainContact *DomainContact) *DNSProvisioner {
	domainClientCfg := awsConfig
	domainClientCfg.Region = "us-east-1"
	return &DNSProvisioner{
		domainContact: domainContact,
		client:        route53.NewFromConfig(awsConfig),
		domainClient:  route53domains.NewFromConfig(domainClientCfg),
		cfClient:      cloudfront.NewFromConfig(awsConfig),
	}
}

func (d *DNSProvisioner) MapCfDistributionToS3(ctx context.Context, sitePath, s3WebDomain, domain, certificateArn string) (string, error) {
	res, err := d.cfClient.CreateDistribution(ctx, &cloudfront.CreateDistributionInput{
		DistributionConfig: &types.DistributionConfig{
			CallerReference: aws.String(time.Now().String()), // must be unique per request, used for idempotency
			Comment:         aws.String("Distribution for site " + sitePath),

			Enabled:           aws.Bool(true),
			DefaultRootObject: aws.String("index.html"),

			Origins: &types.Origins{
				Quantity: aws.Int32(1),
				Items: []types.Origin{
					{
						Id:         aws.String("1"),
						DomainName: aws.String(s3WebDomain),
						OriginPath: aws.String(sitePath),
						CustomOriginConfig: &types.CustomOriginConfig{
							HTTPPort:             aws.Int32(80),
							HTTPSPort:            aws.Int32(443),
							OriginProtocolPolicy: types.OriginProtocolPolicyHttpOnly,
							OriginSslProtocols: &types.OriginSslProtocols{
								Quantity: aws.Int32(1),
								Items:    []types.SslProtocol{types.SslProtocolTLSv12},
							},
						},
					},
				},
			},

			DefaultCacheBehavior: &types.DefaultCacheBehavior{
				TargetOriginId:       aws.String("1"),
				ViewerProtocolPolicy: types.ViewerProtocolPolicyRedirectToHttps,
				AllowedMethods: &types.AllowedMethods{
					Quantity: aws.Int32(2),
					Items:    []types.Method{types.MethodGet, types.MethodHead},
					CachedMethods: &types.CachedMethods{
						Quantity: aws.Int32(2),
						Items:    []types.Method{types.MethodGet, types.MethodHead},
					},
				},
				ForwardedValues: &types.ForwardedValues{
					QueryString: aws.Bool(false),
					Cookies: &types.CookiePreference{
						Forward: types.ItemSelectionNone,
					},
				},
				TrustedSigners: &types.TrustedSigners{
					Enabled:  aws.Bool(false),
					Quantity: aws.Int32(0),
				},
				MinTTL: aws.Int64(0),
			},

			Aliases: &types.Aliases{
				Quantity: aws.Int32(1),
				Items:    []string{domain}, // "test.test-dom-1.click"
			},

			ViewerCertificate: &types.ViewerCertificate{
				ACMCertificateArn:      aws.String(certificateArn), // "arn:aws:acm:us-east-1:123456789012:certificate/your-certificate-id"
				SSLSupportMethod:       types.SSLSupportMethodSniOnly,
				MinimumProtocolVersion: types.MinimumProtocolVersionTLSv122021,
			},

			HttpVersion:   types.HttpVersionHttp2,
			IsIPV6Enabled: aws.Bool(false),
		},
	})
	if err != nil {
		slog.Error("err mapping s3 to cloudfront distr", "cf", err)
		return "", err
	}
	return aws.ToString(res.Distribution.Id), nil
}

func (d *DNSProvisioner) WaitAndGetDistribution(ctx context.Context, distributionID string) (string, error) {
	const (
		maxWaitTime  = 3 * time.Second
		pollInterval = 1 * time.Second
	)

	deadline := time.Now().Add(maxWaitTime)

	for time.Now().Before(deadline) {
		resp, err := d.cfClient.GetDistribution(ctx, &cloudfront.GetDistributionInput{
			Id: &distributionID,
		})
		if err != nil {
			return "", fmt.Errorf("failed to get distribution: %w", err)
		}

		status := *resp.Distribution.Status
		slog.Info("Waiting for deployment", "status", status)

		if status == "Deployed" {
			slog.Info("Distribution is deployed!")
			return aws.ToString(resp.Distribution.DomainName), nil
		}

		time.Sleep(pollInterval)
	}

	return "", fmt.Errorf("timed out waiting for distribution to deploy")
}

func (d *DNSProvisioner) RequestDomain(ctx context.Context, domain string) (string, error) {
	available, err := d.CheckAvailability(domain)
	if err != nil || !available {
		return "", fmt.Errorf("domain %v is not available", domain)
	}
	domainContact := rdTypes.ContactDetail{
		FirstName:    aws.String(d.domainContact.FirstName),
		LastName:     aws.String(d.domainContact.LastName),
		Email:        aws.String(d.domainContact.Email),
		PhoneNumber:  aws.String(d.domainContact.PhoneNumber),
		AddressLine1: aws.String(d.domainContact.AddressLine1),
		City:         aws.String(d.domainContact.City),
		State:        aws.String(d.domainContact.State),
		CountryCode:  rdTypes.CountryCodeUs,
		ZipCode:      aws.String(d.domainContact.ZipCode),
	}
	res, err := d.domainClient.RegisterDomain(ctx, &route53domains.RegisterDomainInput{
		DomainName:                      aws.String(domain),
		DurationInYears:                 aws.Int32(1),
		AutoRenew:                       aws.Bool(true),
		AdminContact:                    &domainContact,
		RegistrantContact:               &domainContact,
		TechContact:                     &domainContact,
		PrivacyProtectAdminContact:      aws.Bool(true),
		PrivacyProtectRegistrantContact: aws.Bool(true),
		PrivacyProtectTechContact:       aws.Bool(true),
	})
	if err != nil {
		return "", err
	}

	return aws.ToString(res.OperationId), nil
}

func (d *DNSProvisioner) GetDomainStatus(operationID string) (rdTypes.OperationStatus, error) {
	res, err := d.domainClient.GetOperationDetail(context.Background(), &route53domains.GetOperationDetailInput{OperationId: aws.String(operationID)})
	if err != nil {
		return "", err
	}

	return res.Status, nil
}

func (d *DNSProvisioner) CheckAvailability(domain string) (bool, error) {
	out, err := d.domainClient.CheckDomainAvailability(context.Background(), &route53domains.CheckDomainAvailabilityInput{
		DomainName: aws.String(domain),
	})
	if err != nil {
		return false, err
	}
	return out.Availability == rdTypes.DomainAvailabilityAvailable, nil
}

func (d *DNSProvisioner) CreateSubdomain(baseDomain, domain, cfDomain string) error {

	res, err := d.client.ListHostedZonesByName(context.Background(), &route53.ListHostedZonesByNameInput{
		DNSName: aws.String(baseDomain),
	})
	if err != nil {
		return err
	}
	var hostedZoneID string
	for _, hostedZone := range res.HostedZones {
		parts := strings.SplitN(aws.ToString(hostedZone.Id), "/hostedzone/", 2)
		hostedZoneID = parts[1]
	}
	fmt.Println(hostedZoneID) // TODO: make sure it's correct

	input := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(hostedZoneID),
		ChangeBatch: &rTypes.ChangeBatch{
			Changes: []rTypes.Change{
				{
					Action: rTypes.ChangeActionUpsert,
					ResourceRecordSet: &rTypes.ResourceRecordSet{
						Name: aws.String(domain),
						Type: rTypes.RRTypeA,
						AliasTarget: &rTypes.AliasTarget{
							DNSName:              aws.String(cfDomain),
							HostedZoneId:         aws.String("Z2FDTNDATAQYW2"),
							EvaluateTargetHealth: false,
						},
					},
				},
			},
		},
	}

	resp, err := d.client.ChangeResourceRecordSets(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to create alias record: %w", err)
	}

	fmt.Println("Record change submitted. Change ID:", aws.ToString(resp.ChangeInfo.Id))
	return nil
}
