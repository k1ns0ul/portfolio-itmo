package recommender

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/andrey/t-vygoda/internal/models"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string, timeout time.Duration) *Client {
	if timeout == 0 {
		timeout = 500 * time.Millisecond
	}
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: timeout},
	}
}

func (c *Client) GetRecommendations(ctx context.Context, userID int64) []models.Recommendation {
	if c.baseURL == "" {
		return nil
	}
	url := c.baseURL + "/recommendations/" + strconv.FormatInt(userID, 10)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	resp, err := c.do(req)
	if err != nil {
		slog.Debug("recommender unreachable", "err", err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var body struct {
		Items []models.Recommendation `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		slog.Debug("recommender decode", "err", err)
		return nil
	}
	return body.Items
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	var last error
	delay := 50 * time.Millisecond
	for i := 0; i < 2; i++ {
		resp, err := c.http.Do(req)
		if err == nil {
			return resp, nil
		}
		last = err
		select {
		case <-time.After(delay):
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
		delay *= 2
	}
	return nil, fmt.Errorf("recommender call: %w", last)
}
