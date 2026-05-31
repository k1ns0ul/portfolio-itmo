package models

type PnLData struct {
	Realized   float64 `json:"realized"`
	Unrealized float64 `json:"unrealized"`
	Total      float64 `json:"total"`
	PctReturn  float64 `json:"pct_return"`
	CostBasis  float64 `json:"cost_basis"`
}

type TokenPnL struct {
	Mint          string  `json:"mint"`
	AvgBuyPrice   float64 `json:"avg_buy_price"`
	CurrentPrice  float64 `json:"current_price"`
	Quantity      float64 `json:"quantity"`
	RealizedPnL   float64 `json:"realized_pnl"`
	UnrealizedPnL float64 `json:"unrealized_pnl"`
}
