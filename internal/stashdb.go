package internal

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
)

type stashDBClient struct {
	apiKey string
	client *http.Client
}

func newStashDBClient(apiKey string) *stashDBClient {
	return &stashDBClient{
		apiKey: apiKey,
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

func (c *stashDBClient) getTagCount() (int, error) {
	query := `query { queryTags(input: { page: 1 }) { count }}`
	var resp struct {
		Data struct {
			QueryTags struct {
				Count int `json:"count"`
			} `json:"queryTags"`
		} `json:"data"`
	}
	if err := c.doQuery(query, &resp); err != nil {
		return 0, err
	}
	return resp.Data.QueryTags.Count, nil
}

func (c *stashDBClient) doQuery(query string, out interface{}) error {
	body, _ := json.Marshal(struct {
		Query string `json:"query"`
	}{Query: query})
	req, err := http.NewRequest("POST", StashDBURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("ApiKey", c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("stashdb API: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
