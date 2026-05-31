package models

type ConcentrationLevel string

const (
	ConcentrationLow    ConcentrationLevel = "low"
	ConcentrationMedium ConcentrationLevel = "medium"
	ConcentrationHigh   ConcentrationLevel = "high"
)

type DiversificationData struct {
	HerfindahlIndex    float64            `json:"herfindahl_index"`
	TokenCount         int                `json:"token_count"`
	StablecoinPct      float64            `json:"stablecoin_pct"`
	BlueChipPct        float64            `json:"blue_chip_pct"`
	TopTokenPct        float64            `json:"top_token_pct"`
	ConcentrationLevel ConcentrationLevel `json:"concentration_level"`
}
