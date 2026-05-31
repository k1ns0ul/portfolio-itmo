package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"

	"github.com/andrey/cfa-bonds/internal/models"
)

const (
	portfolioTTL = 2 * time.Minute
	quoteTTL     = 30 * time.Second
)

func portfolioKey(id uuid.UUID) string { return "portfolio:" + id.String() }
func quoteKey(id uuid.UUID) string     { return "quote:" + id.String() }

func (c *Client) SetPortfolio(ctx context.Context, p *models.PortfolioSummary) error {
	body, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal portfolio %s: %w", p.InvestorID, err)
	}
	if err := c.rdb.Set(ctx, portfolioKey(p.InvestorID), body, portfolioTTL).Err(); err != nil {
		return fmt.Errorf("cache portfolio %s: %w", p.InvestorID, err)
	}
	return nil
}

func (c *Client) GetPortfolio(ctx context.Context, investorID uuid.UUID) (*models.PortfolioSummary, bool, error) {
	raw, err := c.rdb.Get(ctx, portfolioKey(investorID)).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read cached portfolio %s: %w", investorID, err)
	}
	var p models.PortfolioSummary
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, false, fmt.Errorf("decode cached portfolio: %w", err)
	}
	return &p, true, nil
}

func (c *Client) InvalidatePortfolio(ctx context.Context, ids ...uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = portfolioKey(id)
	}
	if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("invalidate %d portfolios: %w", len(ids), err)
	}
	return nil
}

func (c *Client) SetQuote(ctx context.Context, issueID uuid.UUID, price decimal.Decimal) error {
	if err := c.rdb.Set(ctx, quoteKey(issueID), price.String(), quoteTTL).Err(); err != nil {
		return fmt.Errorf("cache quote %s: %w", issueID, err)
	}
	return nil
}

func (c *Client) GetQuote(ctx context.Context, issueID uuid.UUID) (decimal.Decimal, bool, error) {
	raw, err := c.rdb.Get(ctx, quoteKey(issueID)).Result()
	if errors.Is(err, goredis.Nil) {
		return decimal.Zero, false, nil
	}
	if err != nil {
		return decimal.Zero, false, fmt.Errorf("read quote %s: %w", issueID, err)
	}
	d, err := decimal.NewFromString(raw)
	if err != nil {
		return decimal.Zero, false, fmt.Errorf("parse cached quote: %w", err)
	}
	return d, true, nil
}
