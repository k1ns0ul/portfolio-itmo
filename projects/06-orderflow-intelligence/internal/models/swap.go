package models

import "time"

type SwapDirection string

const (
	DirBuy  SwapDirection = "buy"
	DirSell SwapDirection = "sell"
)

type SwapEvent struct {
	Signature   string        `json:"signature"`
	Slot        uint64        `json:"slot"`
	BlockTime   time.Time     `json:"block_time"`
	Dex         string        `json:"dex"`
	PoolAddress string        `json:"pool_address"`
	Pair        string        `json:"pair"`
	TokenIn     string        `json:"token_in"`
	TokenOut    string        `json:"token_out"`
	AmountIn    uint64        `json:"amount_in"`
	AmountOut   uint64        `json:"amount_out"`
	Price       float64       `json:"price"`
	Direction   SwapDirection `json:"direction"`
	Sender      string        `json:"sender"`
}

func (s SwapEvent) IsBuy() bool  { return s.Direction == DirBuy }
func (s SwapEvent) IsSell() bool { return s.Direction == DirSell }

func (s SwapEvent) VolumeIn() float64 { return float64(s.AmountIn) }
