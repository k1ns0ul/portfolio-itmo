package models

type TokenPosition struct {
	Mint           string  `json:"mint"`
	Symbol         string  `json:"symbol"`
	Amount         float64 `json:"amount"`
	ValueUSD       float64 `json:"value_usd"`
	PctOfPortfolio float64 `json:"pct_of_portfolio"`
	PnL            float64 `json:"pnl"`
	PnLPct         float64 `json:"pnl_pct"`
	IsStablecoin   bool    `json:"is_stablecoin"`
	IsBlueChip     bool    `json:"is_blue_chip"`
	RiskCategory   string  `json:"risk_category"`
}

var StablecoinMints = map[string]string{
	"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v": "USDC",
	"Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB": "USDT",
	"2b1kV6DkPAnxd5ixfnxCpjxmKwqjjaYmCZfHsFu24GXo": "PYUSD",
	"HzwqbKZw8HxMN6bF2yFZNrht3c2iXXzpKcFu7uBEDKtr": "EURC",
}

var BlueChipMints = map[string]string{
	"So11111111111111111111111111111111111111112":  "SOL",
	"7vfCXTUXx5WJV5JADk17DUJ4ksgau7utNKj4b963voxs": "ETH",
	"3NZ9JMVBmGAqocybic2c7LQCJScmgsAZ6vQqTDzcqmJh": "WBTC",
}
