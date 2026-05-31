package solana

import "encoding/base64"

const (
	SystemProgram          = "11111111111111111111111111111111"
	TokenProgram           = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	TokenProgram2022       = "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb"
	AssociatedTokenProgram = "ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL"
	RaydiumAMMv4           = "675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8"
	RaydiumCPMM            = "CPMMoo8L3F4NbTegBCKVNunggL7H1ZpdTHKxQB5qKP1C"
	OrcaWhirlpool          = "whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc"
	JupiterV6              = "JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4"
	MeteoraDLMM            = "LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo"
	ComputeBudgetProgram   = "ComputeBudget111111111111111111111111111111"
	MemoProgram            = "MemoSq4gqABAXKb96qnH8TysNcWxMyWCqXgDLGmfcHr"
)

type InstructionKind string

const (
	KindUnknown       InstructionKind = "unknown"
	KindTransfer      InstructionKind = "transfer"
	KindSwap          InstructionKind = "swap"
	KindCreateAccount InstructionKind = "create_account"
	KindCloseAccount  InstructionKind = "close_account"
	KindMint          InstructionKind = "mint"
	KindBurn          InstructionKind = "burn"
	KindMemo          InstructionKind = "memo"
	KindCompute       InstructionKind = "compute"
)

func TrackedPrograms() []string {
	return []string{
		SystemProgram, TokenProgram, TokenProgram2022, AssociatedTokenProgram,
		RaydiumAMMv4, RaydiumCPMM, OrcaWhirlpool, JupiterV6, MeteoraDLMM,
	}
}

func IsSmartContract(programID string) bool {
	switch programID {
	case SystemProgram, TokenProgram, TokenProgram2022, AssociatedTokenProgram, MemoProgram, ComputeBudgetProgram:
		return false
	default:
		return true
	}
}

func IsDEX(programID string) bool {
	switch programID {
	case RaydiumAMMv4, RaydiumCPMM, OrcaWhirlpool, JupiterV6, MeteoraDLMM:
		return true
	}
	return false
}

func ClassifyInstruction(programID string, data []byte) InstructionKind {
	switch programID {
	case ComputeBudgetProgram:
		return KindCompute
	case MemoProgram:
		return KindMemo
	case SystemProgram:
		return classifySystem(data)
	case TokenProgram, TokenProgram2022:
		return classifyToken(data)
	case RaydiumAMMv4, RaydiumCPMM, OrcaWhirlpool, JupiterV6, MeteoraDLMM:
		return KindSwap
	}
	return KindUnknown
}

func ClassifyInstructionBase64(programID, dataB64 string) InstructionKind {
	if dataB64 == "" {
		return ClassifyInstruction(programID, nil)
	}
	raw, err := base64.StdEncoding.DecodeString(dataB64)
	if err != nil {
		return KindUnknown
	}
	return ClassifyInstruction(programID, raw)
}

func classifySystem(data []byte) InstructionKind {
	if len(data) < 4 {
		return KindUnknown
	}
	switch leU32(data) {
	case 0:
		return KindCreateAccount
	case 2:
		return KindTransfer
	case 8:
		return KindCloseAccount
	}
	return KindUnknown
}

func classifyToken(data []byte) InstructionKind {
	if len(data) == 0 {
		return KindUnknown
	}
	switch data[0] {
	case 3, 12:
		return KindTransfer
	case 7:
		return KindMint
	case 8:
		return KindBurn
	case 9:
		return KindCloseAccount
	case 1:
		return KindCreateAccount
	}
	return KindUnknown
}

func leU32(b []byte) uint32 {
	_ = b[3]
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}
