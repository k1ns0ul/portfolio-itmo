package models

import "time"

type AccountMeta struct {
	PubKey   string `json:"pubkey"`
	Signer   bool   `json:"signer"`
	Writable bool   `json:"writable"`
}

type Instruction struct {
	ProgramID string   `json:"program_id"`
	Accounts  []string `json:"accounts"`
	Data      string   `json:"data"`
	Kind      string   `json:"kind,omitempty"`
}

type Transaction struct {
	Signature    string        `json:"signature"`
	Slot         uint64        `json:"slot"`
	BlockTime    time.Time     `json:"block_time"`
	Fee          uint64        `json:"fee"`
	Success      bool          `json:"success"`
	Accounts     []string      `json:"accounts"`
	Instructions []Instruction `json:"instructions"`
	Sender       string        `json:"sender"`
	Receiver     string        `json:"receiver"`
	Amount       uint64        `json:"amount"`
	ProgramID    string        `json:"program_id"`
	SwapKind     string        `json:"swap_kind,omitempty"`
	RawData      string        `json:"raw_data,omitempty"`
}

type SwapEvent struct {
	DEX         string `json:"dex"`
	Pool        string `json:"pool"`
	User        string `json:"user"`
	TokenIn     string `json:"token_in"`
	TokenOut    string `json:"token_out"`
	AmountIn    uint64 `json:"amount_in"`
	MinOut      uint64 `json:"min_out"`
	AmountOut   uint64 `json:"amount_out,omitempty"`
	Slippage    uint16 `json:"slippage_bps,omitempty"`
	IsBuy       bool   `json:"is_buy"`
	Signature   string `json:"signature"`
	Slot        uint64 `json:"slot"`
	IxIndex     int    `json:"ix_index"`
}

func (t Transaction) Counterparty(addr string) string {
	switch {
	case t.Sender == addr:
		return t.Receiver
	case t.Receiver == addr:
		return t.Sender
	}
	return ""
}

func (t Transaction) Involves(addr string) bool {
	if t.Sender == addr || t.Receiver == addr {
		return true
	}
	for _, a := range t.Accounts {
		if a == addr {
			return true
		}
	}
	return false
}
