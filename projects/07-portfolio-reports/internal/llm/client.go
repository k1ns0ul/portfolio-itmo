package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/andrey/portfolio-reports/internal/models"
)

type Client struct {
	baseURL  string
	http     *http.Client
	retries  int
	fallback *FallbackRenderer
}

type GenerateResponse struct {
	Text    string `json:"text"`
	Summary string `json:"summary"`
	Source  string `json:"source"`
}

func NewClient(baseURL string, timeout time.Duration, retries int) (*Client, error) {
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	if retries < 0 {
		retries = 0
	}
	fb, err := NewFallbackRenderer()
	if err != nil {
		return nil, fmt.Errorf("fallback: %w", err)
	}
	return &Client{
		baseURL:  baseURL,
		http:     &http.Client{Timeout: timeout},
		retries:  retries,
		fallback: fb,
	}, nil
}

func (c *Client) Generate(ctx context.Context, p *models.Portfolio) (*GenerateResponse, error) {
	if c.baseURL != "" {
		resp, err := c.callLLM(ctx, p)
		if err == nil {
			return resp, nil
		}
		slog.Warn("llm unreachable, using template fallback", "err", err)
	}
	return c.renderFallback(p)
}

func (c *Client) callLLM(ctx context.Context, p *models.Portfolio) (*GenerateResponse, error) {
	body, err := json.Marshal(map[string]any{"portfolio": p})
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	var lastErr error
	delay := 500 * time.Millisecond
	for attempt := 0; attempt <= c.retries; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/generate", bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			c.backoff(ctx, &delay)
			continue
		}
		out, parseErr := parseLLMResponse(resp)
		if parseErr != nil {
			lastErr = parseErr
			c.backoff(ctx, &delay)
			continue
		}
		return out, nil
	}
	return nil, fmt.Errorf("llm call failed after %d attempts: %w", c.retries+1, lastErr)
}

func parseLLMResponse(resp *http.Response) (*GenerateResponse, error) {
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("llm 5xx: %s", string(raw))
	}
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("llm status %d: %s", resp.StatusCode, string(raw))
	}
	var out GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if out.Text == "" {
		return nil, errors.New("empty llm text")
	}
	if out.Source == "" {
		out.Source = "llm"
	}
	return &out, nil
}

func (c *Client) backoff(ctx context.Context, delay *time.Duration) {
	select {
	case <-time.After(*delay):
	case <-ctx.Done():
		return
	}
	*delay *= 2
	if *delay > 5*time.Second {
		*delay = 5 * time.Second
	}
}

func (c *Client) renderFallback(p *models.Portfolio) (*GenerateResponse, error) {
	text, summary, err := c.fallback.Render(p)
	if err != nil {
		return nil, fmt.Errorf("fallback render: %w", err)
	}
	return &GenerateResponse{Text: text, Summary: summary, Source: "template"}, nil
}
