package templater

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type TemplaterClient struct {
	cfg    *TemplaterConfig
	client http.Client
}

func NewTemplaterClient(config *TemplaterConfig) *TemplaterClient {
	return &TemplaterClient{
		config,
		http.Client{Timeout: 4 * time.Second},
	}
}

func (c *TemplaterClient) ProvisionSite(req ProvisionSiteRequest) (int, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return 0, err
	}
	request, err := http.NewRequest("POST", c.getURL()+"/provision", bytes.NewBuffer(body))
	if err != nil {
		return 0, err
	}
	resp, err := c.client.Do(request)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result int
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return 0, err
	}

	return result, nil
}

func (c *TemplaterClient) HealthCheckSite(siteID uint64) (HealthcheckProvisionResponseStatus, error) {
	req := HealthcheckProvisionRequest{SiteID: siteID}
	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	request, err := http.NewRequest("POST", c.getURL()+"/provision/healthcheck", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	resp, err := c.client.Do(request)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result HealthcheckProvisionResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", err
	}

	return result.Status, nil
}

func (c *TemplaterClient) getURL() string {
	return fmt.Sprintf("%v://%v:%v", c.cfg.schema, c.cfg.host, c.cfg.port)
}
