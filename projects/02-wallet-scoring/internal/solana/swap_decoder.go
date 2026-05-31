package solana

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/andrey/wallet-scoring/internal/models"
)

var (
	raydiumV4SwapBaseIn    = []byte{9}
	raydiumV4SwapBaseOut   = []byte{11}
	orcaWhirlpoolSwap      = []byte{0xf8, 0xc6, 0x9e, 0x91, 0xe1, 0x75, 0x87, 0xc8}
	orcaWhirlpoolSwapV2    = []byte{0x2b, 0x04, 0xed, 0x0b, 0x1a, 0xc9, 0x1e, 0x62}
	jupiterRouteV6         = []byte{0xe5, 0x17, 0xcb, 0x97, 0x7a, 0xe3, 0xad, 0x2a}
	jupiterSharedAccounts  = []byte{0xc1, 0x20, 0x9b, 0x33, 0x41, 0xd6, 0x9c, 0x81}
)

func DecodeSwap(ix models.Instruction, signature string, slot uint64, ixIndex int) (*models.SwapEvent, bool) {
	data, err := DecodeBase64Data(ix.Data)
	if err != nil {
		return nil, false
	}
	switch ix.ProgramID {
	case RaydiumAMMv4:
		return decodeRaydiumV4(data, ix.Accounts, signature, slot, ixIndex)
	case OrcaWhirlpool:
		return decodeOrca(data, ix.Accounts, signature, slot, ixIndex)
	case JupiterV6:
		return decodeJupiter(data, ix.Accounts, signature, slot, ixIndex)
	}
	return nil, false
}

func decodeRaydiumV4(data []byte, accounts []string, sig string, slot uint64, ixIndex int) (*models.SwapEvent, bool) {
	if len(data) < 17 || len(accounts) < 17 {
		return nil, false
	}
	var amountIn, minOut uint64
	switch {
	case bytes.HasPrefix(data, raydiumV4SwapBaseIn):
		amountIn = binary.LittleEndian.Uint64(data[1:9])
		minOut = binary.LittleEndian.Uint64(data[9:17])
	case bytes.HasPrefix(data, raydiumV4SwapBaseOut):
		minOut = binary.LittleEndian.Uint64(data[1:9])
		amountIn = binary.LittleEndian.Uint64(data[9:17])
	default:
		return nil, false
	}
	return &models.SwapEvent{
		DEX:       "raydium-v4",
		Pool:      accounts[1],
		User:      accounts[16],
		TokenIn:   accounts[14],
		TokenOut:  accounts[15],
		AmountIn:  amountIn,
		MinOut:    minOut,
		Signature: sig,
		Slot:      slot,
		IxIndex:   ixIndex,
	}, true
}

func decodeOrca(data []byte, accounts []string, sig string, slot uint64, ixIndex int) (*models.SwapEvent, bool) {
	if len(accounts) < 11 {
		return nil, false
	}
	var amountIn, minOut uint64
	var aToB bool
	if bytes.HasPrefix(data, orcaWhirlpoolSwap) {
		if len(data) < 8+8+8+16+1+1 {
			return nil, false
		}
		amountIn = binary.LittleEndian.Uint64(data[8:16])
		minOut = binary.LittleEndian.Uint64(data[16:24])
		aToB = data[len(data)-1] != 0
	} else if bytes.HasPrefix(data, orcaWhirlpoolSwapV2) {
		if len(data) < 8+8+8 {
			return nil, false
		}
		amountIn = binary.LittleEndian.Uint64(data[8:16])
		minOut = binary.LittleEndian.Uint64(data[16:24])
	} else {
		return nil, false
	}
	tokenIn, tokenOut := accounts[2], accounts[4]
	if !aToB {
		tokenIn, tokenOut = accounts[4], accounts[2]
	}
	return &models.SwapEvent{
		DEX:       "orca-whirlpool",
		Pool:      accounts[1],
		User:      accounts[0],
		TokenIn:   tokenIn,
		TokenOut:  tokenOut,
		AmountIn:  amountIn,
		MinOut:    minOut,
		IsBuy:     aToB,
		Signature: sig,
		Slot:      slot,
		IxIndex:   ixIndex,
	}, true
}

func decodeJupiter(data []byte, accounts []string, sig string, slot uint64, ixIndex int) (*models.SwapEvent, bool) {
	switch {
	case bytes.HasPrefix(data, jupiterRouteV6), bytes.HasPrefix(data, jupiterSharedAccounts):
	default:
		return nil, false
	}
	if len(accounts) < 13 || len(data) < 8+1+8+8+2 {
		return nil, false
	}
	body := data[8:]
	routeLen := int(body[0])
	cursor := 1 + routeLen*40
	if len(body) < cursor+8+8+2 {
		return nil, false
	}
	amountIn := binary.LittleEndian.Uint64(body[cursor : cursor+8])
	minOut := binary.LittleEndian.Uint64(body[cursor+8 : cursor+16])
	slippage := binary.LittleEndian.Uint16(body[cursor+16 : cursor+18])
	return &models.SwapEvent{
		DEX:       "jupiter-v6",
		User:      accounts[1],
		TokenIn:   accounts[3],
		TokenOut:  accounts[6],
		AmountIn:  amountIn,
		MinOut:    minOut,
		Slippage:  slippage,
		Signature: sig,
		Slot:      slot,
		IxIndex:   ixIndex,
	}, true
}

func DecodeAllSwaps(tx *models.Transaction) []models.SwapEvent {
	var out []models.SwapEvent
	for i, ix := range tx.Instructions {
		if !IsDEX(ix.ProgramID) {
			continue
		}
		ev, ok := DecodeSwap(ix, tx.Signature, tx.Slot, i)
		if !ok {
			continue
		}
		out = append(out, *ev)
		if i == 0 {
			tx.SwapKind = ev.DEX
		}
	}
	return out
}

var ErrUnknownDEX = errors.New("unknown DEX")
