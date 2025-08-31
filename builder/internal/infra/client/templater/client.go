package templater

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"time"
)

type TemplaterClient struct {
	cfg    *TemplaterConfig
	client *http.Client
}

func NewTemplaterClient(config *TemplaterConfig) *TemplaterClient {
	return &TemplaterClient{
		config,
		&http.Client{Timeout: 4 * time.Second,
			Transport: &http.Transport{
				// This forces DNS/TCP retries to log
				Proxy: http.ProxyFromEnvironment,
			}},
	}
}

func (c *TemplaterClient) ProvisionSite(ctx context.Context, req ProvisionSiteRequest) (uint64, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return 0, err
	}
	request, err := http.NewRequestWithContext(ctx, "POST", c.getURL()+"/provision", bytes.NewBuffer(body))
	if err != nil {
		return 0, err
	}
	request.Header.Set("Content-Type", "application/json")
	dump, _ := httputil.DumpRequestOut(request, true)
	fmt.Println(string(dump))
	resp, err := c.client.Do(request)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result ProvisionSiteResponse
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	return result.SiteID, nil
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
