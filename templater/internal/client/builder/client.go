package builder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type BuilderClient struct {
	cfg    *BuilderConfig
	client http.Client
}

func NewBuilderClient(config *BuilderConfig) *BuilderClient {
	return &BuilderClient{
		config,
		http.Client{Timeout: 4 * time.Second},
	}
}

func (c *BuilderClient) UpdateSite(siteID uint64, req UpdateSiteRequest) (uint64, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return 0, err
	}
	request, err := http.NewRequest("PATCH", fmt.Sprintf("%v/sites/%d", c.getURL(), siteID), bytes.NewBuffer(body))
	if err != nil {
		return 0, err
	}
	resp, err := c.client.Do(request)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result UpdateSiteResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return 0, err
	}

	return result.SiteID, nil
}

func (c *BuilderClient) getURL() string {
	return fmt.Sprintf("%v://%v:%v", c.cfg.schema, c.cfg.host, c.cfg.port)
}
