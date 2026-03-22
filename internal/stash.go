package internal

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
)

type stashClient struct {
	url    string
	apiKey string
	client *http.Client
}

func newStashClient(url, apiKey string) *stashClient {
	return &stashClient{
		url:    url,
		apiKey: apiKey,
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

type graphqlReq struct {
	Query string `json:"query"`
}

type graphqlResp struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}

type stashTag struct {
	Name          string   `json:"name"`
	Aliases       []string `json:"aliases"`
	ImagePath     string   `json:"image_path"`
	ID            string   `json:"id"`
	IgnoreAutoTag bool     `json:"ignore_auto_tag"`
	StashIDs      []struct {
		StashID  string `json:"stash_id"`
		Endpoint string `json:"endpoint"`
	} `json:"stash_ids"`
}

type findTagsResp struct {
	FindTags struct {
		Tags []stashTag `json:"tags"`
	} `json:"findTags"`
}

func (c *stashClient) getAllTags() ([]stashTag, error) {
	query := `query {
  findTags(filter: { per_page: -1 }) {
    tags {
      name aliases image_path id ignore_auto_tag
      stash_ids { stash_id endpoint }
    }
  }
}`
	var resp findTagsResp
	if err := c.doQuery(query, &resp); err != nil {
		return nil, err
	}
	return resp.FindTags.Tags, nil
}

func (c *stashClient) doQuery(query string, out interface{}) error {
	body, _ := json.Marshal(graphqlReq{Query: query})
	req, err := http.NewRequest("POST", c.url, bytes.NewReader(body))
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
		return fmt.Errorf("stash API: %s", resp.Status)
	}

	var gql graphqlResp
	if err := json.NewDecoder(resp.Body).Decode(&gql); err != nil {
		return err
	}
	if len(gql.Errors) > 0 {
		return fmt.Errorf("graphql: %s", gql.Errors[0].Message)
	}
	return json.Unmarshal(gql.Data, out)
}
