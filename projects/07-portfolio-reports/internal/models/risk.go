package models

type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type RiskData struct {
	Score        float64   `json:"score"`
	Level        RiskLevel `json:"level"`
	VolatilePct  float64   `json:"volatile_pct"`
	ScamTokenPct float64   `json:"scam_token_pct"`
	Factors      []string  `json:"factors"`
}
