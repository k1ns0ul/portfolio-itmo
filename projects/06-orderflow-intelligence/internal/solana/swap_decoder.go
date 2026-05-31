package solana

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/andrey/orderflow-intelligence/internal/models"
)

const (
	RaydiumV4     = "675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8"
	RaydiumCPMM   = "CPMMoo8L3F4NbTegBCKVNunggL7H1ZpdTHKxQB5qKP1C"
	OrcaWhirlpool = "whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc"
	JupiterV6     = "JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4"
)

var (
	orcaSwapDiscriminator   = []byte{0xf8, 0xc6, 0x9e, 0x91, 0xe1, 0x75, 0x87, 0xc8}
	orcaSwapV2Discriminator = []byte{0x2b, 0x04, 0xed, 0x0b, 0x1a, 0xc9, 0x1e, 0x62}
	jupiterRouteV6          = []byte{0xe5, 0x17, 0xcb, 0x97, 0x7a, 0xe3, 0xad, 0x2a}
	jupiterSharedAccounts   = []byte{0xc1, 0x20, 0x9b, 0x33, 0x41, 0xd6, 0x9c, 0x81}
)

var ErrUnknownDEX = errors.New("unknown DEX")

func IsDEX(programID string) bool {
	switch programID {
	case RaydiumV4, RaydiumCPMM, OrcaWhirlpool, JupiterV6:
		return true
	}
	return false
}

func DEXName(programID string) string {
	switch programID {
	case RaydiumV4:
		return "raydium-v4"
	case RaydiumCPMM:
		return "raydium-cpmm"
	case OrcaWhirlpool:
		return "orca-whirlpool"
	case JupiterV6:
		return "jupiter-v6"
	}
	return "unknown"
}

func DecodeSwap(programID string, data []byte, accounts []string) (*models.SwapEvent, error) {
	switch programID {
	case RaydiumV4:
		return decodeRaydiumV4(data, accounts)
	case OrcaWhirlpool:
		return decodeOrcaWhirlpool(data, accounts)
	case JupiterV6:
		return decodeJupiter(data, accounts)
	}
	return nil, ErrUnknownDEX
}

func TryDecodeSwaps(instructions []Instruction, _ []string) []models.SwapEvent {
	out := make([]models.SwapEvent, 0, 1)
	for _, ix := range instructions {
		if !IsDEX(ix.ProgramID) {
			continue
		}
		ev, err := DecodeSwap(ix.ProgramID, ix.RawData, ix.Accounts)
		if err != nil {
			continue
		}
		ev.Dex = DEXName(ix.ProgramID)
		if ev.BlockTime.IsZero() {
			ev.BlockTime = time.Now().UTC()
		}
		out = append(out, *ev)
	}
	return out
}

func decodeRaydiumV4(data []byte, accounts []string) (*models.SwapEvent, error) {
	if len(data) < 17 || len(accounts) < 18 {
		return nil, fmt.Errorf("raydium: short payload data=%d acc=%d", len(data), len(accounts))
	}
	if data[0] != 9 && data[0] != 11 {
		return nil, fmt.Errorf("raydium: not a swap instruction byte=%d", data[0])
	}
	amountIn := binary.LittleEndian.Uint64(data[1:9])
	amountOut := binary.LittleEndian.Uint64(data[9:17])

	poolCoin := accounts[5]
	userSource := accounts[15]
	userDest := accounts[16]
	owner := accounts[17]

	direction := models.DirBuy
	if userSource == poolCoin {
		direction = models.DirSell
	}

	pair := simplePair(userSource, userDest)
	price := float64(amountOut) / nonZero(float64(amountIn))
	return &models.SwapEvent{
		PoolAddress: accounts[1],
		Pair:        pair,
		TokenIn:     userSource,
		TokenOut:    userDest,
		AmountIn:    amountIn,
		AmountOut:   amountOut,
		Price:       price,
		Direction:   direction,
		Sender:      owner,
	}, nil
}

func decodeOrcaWhirlpool(data []byte, accounts []string) (*models.SwapEvent, error) {
	if len(accounts) < 11 {
		return nil, fmt.Errorf("orca: short accounts=%d", len(accounts))
	}
	if len(data) < 8 {
		return nil, fmt.Errorf("orca: short data=%d", len(data))
	}
	if !(bytes.HasPrefix(data, orcaSwapDiscriminator) || bytes.HasPrefix(data, orcaSwapV2Discriminator)) {
		return nil, fmt.Errorf("orca: discriminator mismatch")
	}
	body := data[8:]
	if len(body) < 8+8+16+1+1 {
		return nil, fmt.Errorf("orca: short body=%d", len(body))
	}
	amount := binary.LittleEndian.Uint64(body[0:8])
	threshold := binary.LittleEndian.Uint64(body[8:16])
	amountSpecifiedIsInput := body[len(body)-2] != 0
	aToB := body[len(body)-1] != 0

	var amountIn, amountOut uint64
	if amountSpecifiedIsInput {
		amountIn = amount
		amountOut = threshold
	} else {
		amountIn = threshold
		amountOut = amount
	}

	tokenIn := accounts[4]
	tokenOut := accounts[5]
	if !aToB {
		tokenIn, tokenOut = accounts[5], accounts[4]
	}
	direction := models.DirBuy
	if !aToB {
		direction = models.DirSell
	}
	return &models.SwapEvent{
		PoolAddress: accounts[3],
		Pair:        simplePair(tokenIn, tokenOut),
		TokenIn:     tokenIn,
		TokenOut:    tokenOut,
		AmountIn:    amountIn,
		AmountOut:   amountOut,
		Price:       float64(amountOut) / nonZero(float64(amountIn)),
		Direction:   direction,
		Sender:      accounts[0],
	}, nil
}

func decodeJupiter(data []byte, accounts []string) (*models.SwapEvent, error) {
	if len(accounts) < 7 {
		return nil, fmt.Errorf("jupiter: short accounts=%d", len(accounts))
	}
	if len(data) < 8 {
		return nil, fmt.Errorf("jupiter: short data=%d", len(data))
	}
	if !(bytes.HasPrefix(data, jupiterRouteV6) || bytes.HasPrefix(data, jupiterSharedAccounts)) {
		return nil, fmt.Errorf("jupiter: discriminator mismatch")
	}
	body := data[8:]
	if len(body) < 1+16 {
		return nil, fmt.Errorf("jupiter: short body=%d", len(body))
	}
	routeLen := int(body[0])
	cursor := 1 + routeLen*40
	if len(body) < cursor+16 {
		return nil, fmt.Errorf("jupiter: route truncated")
	}
	amountIn := binary.LittleEndian.Uint64(body[cursor : cursor+8])
	amountOut := binary.LittleEndian.Uint64(body[cursor+8 : cursor+16])

	tokenIn := accounts[2]
	tokenOut := accounts[3]
	if len(accounts) > 6 {
		tokenOut = accounts[6]
	}
	owner := accounts[1]
	return &models.SwapEvent{
		PoolAddress: accounts[2],
		Pair:        simplePair(tokenIn, tokenOut),
		TokenIn:     tokenIn,
		TokenOut:    tokenOut,
		AmountIn:    amountIn,
		AmountOut:   amountOut,
		Price:       float64(amountOut) / nonZero(float64(amountIn)),
		Direction:   models.DirBuy,
		Sender:      owner,
	}, nil
}

func simplePair(a, b string) string {
	return shortMint(a) + "-" + shortMint(b)
}

func shortMint(m string) string {
	if len(m) <= 8 {
		return m
	}
	return m[:4] + ".." + m[len(m)-4:]
}

func nonZero(x float64) float64 {
	if x == 0 {
		return 1
	}
	return x
}
