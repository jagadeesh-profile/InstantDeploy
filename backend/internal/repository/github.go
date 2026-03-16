package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"instantdeploy/backend/pkg/models"
)

type GitHubClient struct {
	token  string
	client *http.Client
}

func NewGitHubClient(token string) *GitHubClient {
	return &GitHubClient{
		token: token,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (g *GitHubClient) Search(ctx context.Context, query string) ([]models.Repository, error) {
	endpoint := fmt.Sprintf("https://api.github.com/search/repositories?q=%s&per_page=10", url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("github search failed: %s", resp.Status)
	}

	var payload struct {
		Items []struct {
			FullName    string `json:"full_name"`
			Description string `json:"description"`
			HTMLURL     string `json:"html_url"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	out := make([]models.Repository, 0, len(payload.Items))
	for _, item := range payload.Items {
		out = append(out, models.Repository{
			Name:        item.FullName,
			Description: item.Description,
			URL:         item.HTMLURL,
		})
	}

	return out, nil
}
