package dns

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/route53"
)

type DNSProvisioner struct {
	client *route53.Client
}

func NewDNSProvisioner() *DNSProvisioner {
	return &DNSProvisioner{
		client: initClient(),
	}
}

func initClient() *route53.Client {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		fmt.Println(err)
	}
	client := route53.NewFromConfig(cfg)
	return client
}
