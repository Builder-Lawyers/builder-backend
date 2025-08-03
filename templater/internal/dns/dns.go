package dns

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	rTypes "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/aws-sdk-go-v2/service/route53domains"
	"log/slog"
	"time"
)

type DNSProvisioner struct {
	client       *route53.Client
	domainClient *route53domains.Client
	cfClient     *cloudfront.Client
}

func NewDNSProvisioner(awsConfig aws.Config) *DNSProvisioner {
	return &DNSProvisioner{
		client:       route53.NewFromConfig(awsConfig),
		domainClient: route53domains.NewFromConfig(awsConfig),
		cfClient:     cloudfront.NewFromConfig(awsConfig),
	}
}

func (d *DNSProvisioner) MapCfDistributionToS3(sitePath, s3WebDomain, domain, certificateArn string) (string, error) {
	res, err := d.cfClient.CreateDistribution(context.TODO(), &cloudfront.CreateDistributionInput{
		DistributionConfig: &types.DistributionConfig{
			CallerReference: aws.String("unique-string-12345"), // must be unique per request, used for idempotency
			Comment:         aws.String("Website for test.test-dom-1.click"),

			Enabled:           aws.Bool(true),
			DefaultRootObject: aws.String("index.html"),

			Origins: &types.Origins{
				Quantity: aws.Int32(1),
				Items: []types.Origin{
					{
						Id:         aws.String("1"),
						DomainName: aws.String(s3WebDomain), // sanity-web.s3-website.eu-north-1.amazonaws.com
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
		return "", err
	}
	return *res.Distribution.Id, nil
}

func (d *DNSProvisioner) WaitAndGetDistribution(ctx context.Context, distributionID string) (string, error) {
	const (
		maxWaitTime  = 2 * time.Minute
		pollInterval = 3 * time.Second
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
		fmt.Println("Waiting for deployment, current status:", status)

		if status == "Deployed" {
			fmt.Println("Distribution is deployed!")
			return *resp.Distribution.DomainName, nil
		}

		time.Sleep(pollInterval)
	}

	return "", fmt.Errorf("timed out waiting for distribution to deploy")
}

func (d *DNSProvisioner) RequestDomain(domain string) error {
	available, err := d.CheckAvailability(domain)
	if err != nil || !available {
		return fmt.Errorf("domain %v is not available", domain)
	}

	return nil
}

func (d *DNSProvisioner) CheckAvailability(domain string) (bool, error) {
	out, err := d.domainClient.CheckDomainAvailability(context.Background(), &route53domains.CheckDomainAvailabilityInput{
		DomainName: aws.String(domain),
	})
	if err != nil {
		return false, err
	}
	slog.Info("DNS", out)
	return false, nil
}

func (d *DNSProvisioner) CreateSubdomain(domain, domainID, cfDomain string) error {

	input := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(domainID),
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

	fmt.Println("Record change submitted. Change ID:", *resp.ChangeInfo.Id)
	return nil
}
