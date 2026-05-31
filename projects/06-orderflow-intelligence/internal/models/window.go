package models

import "time"

type FeatureWindow struct {
	Pair             string    `json:"pair"`
	IntervalSec      int       `json:"interval_sec"`
	WindowStart      time.Time `json:"window_start"`
	WindowEnd        time.Time `json:"window_end"`
	OFI              float64   `json:"ofi"`
	VPIN             float64   `json:"vpin"`
	PriceImpact      float64   `json:"price_impact"`
	AvgSwapSize      float64   `json:"avg_swap_size"`
	BuyRatio         float64   `json:"buy_ratio"`
	CumulativeVolume float64   `json:"cumulative_volume"`
	PriceRange       float64   `json:"price_range"`
	PriceOpen        float64   `json:"price_open"`
	PriceClose       float64   `json:"price_close"`
	SwapCount        int       `json:"swap_count"`
}
