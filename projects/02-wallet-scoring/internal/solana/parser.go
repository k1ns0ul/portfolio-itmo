package solana

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/andrey/wallet-scoring/internal/models"
)

func ParseTransaction(raw []byte) (*models.Transaction, error) {
	r := &reader{buf: raw}

	sigCount := r.compactU16()
	if r.err != nil || sigCount == 0 || sigCount > 64 {
		return nil, fmt.Errorf("bad signature count: %d", sigCount)
	}
	sigs := make([][]byte, sigCount)
	for i := 0; i < sigCount; i++ {
		sigs[i] = append([]byte(nil), r.bytes(64)...)
	}
	if r.err != nil {
		return nil, r.err
	}

	first := r.peek()
	versioned := first&0x80 != 0
	if versioned {
		_ = r.uint8()
	}

	headerRequiredSigs := r.uint8()
	headerReadonlySigned := r.uint8()
	headerReadonlyUnsigned := r.uint8()
	_, _, _ = headerRequiredSigs, headerReadonlySigned, headerReadonlyUnsigned

	keyCount := r.compactU16()
	if r.err != nil || keyCount == 0 || keyCount > 256 {
		return nil, fmt.Errorf("bad account key count: %d", keyCount)
	}
	keys := make([]string, keyCount)
	for i := 0; i < keyCount; i++ {
		keys[i] = Base58Encode(r.bytes(32))
	}

	_ = r.bytes(32)

	instrCount := r.compactU16()
	if r.err != nil {
		return nil, r.err
	}
	instrs := make([]models.Instruction, 0, instrCount)
	for i := 0; i < instrCount; i++ {
		programIdx := int(r.uint8())
		accCount := r.compactU16()
		accIdx := append([]byte(nil), r.bytes(accCount)...)
		dataLen := r.compactU16()
		data := append([]byte(nil), r.bytes(dataLen)...)
		if r.err != nil {
			return nil, r.err
		}
		if programIdx >= len(keys) {
			return nil, fmt.Errorf("program index %d out of range", programIdx)
		}
		accounts := make([]string, len(accIdx))
		for j, idx := range accIdx {
			if int(idx) >= len(keys) {
				return nil, fmt.Errorf("account index %d out of range", idx)
			}
			accounts[j] = keys[idx]
		}
		progID := keys[programIdx]
		instrs = append(instrs, models.Instruction{
			ProgramID: progID,
			Accounts:  accounts,
			Data:      base64.StdEncoding.EncodeToString(data),
			Kind:      string(ClassifyInstruction(progID, data)),
		})
	}

	if versioned {
		tableCount := r.compactU16()
		for i := 0; i < tableCount; i++ {
			_ = r.bytes(32)
			writeCount := r.compactU16()
			_ = r.bytes(writeCount)
			readCount := r.compactU16()
			_ = r.bytes(readCount)
			if r.err != nil {
				return nil, r.err
			}
		}
	}

	tx := &models.Transaction{
		Signature:    Base58Encode(sigs[0]),
		Accounts:     keys,
		Success:      true,
		Instructions: instrs,
		BlockTime:    time.Now().UTC(),
	}
	fillSummary(tx)
	return tx, nil
}

func fillSummary(tx *models.Transaction) {
	for i := range tx.Instructions {
		ix := tx.Instructions[i]
		if ix.ProgramID == ComputeBudgetProgram || ix.ProgramID == MemoProgram {
			continue
		}
		tx.ProgramID = ix.ProgramID
		if len(ix.Accounts) > 0 {
			tx.Sender = ix.Accounts[0]
		}
		if len(ix.Accounts) > 1 {
			tx.Receiver = ix.Accounts[1]
		}
		tx.RawData = ix.Data
		break
	}
	for _, ix := range tx.Instructions {
		amount, ok := decodeTransferAmount(ix)
		if !ok {
			continue
		}
		tx.Amount = amount
		tx.ProgramID = ix.ProgramID
		if len(ix.Accounts) > 0 {
			tx.Sender = ix.Accounts[0]
		}
		if len(ix.Accounts) > 1 {
			tx.Receiver = ix.Accounts[1]
		}
		return
	}
}

func decodeTransferAmount(ix models.Instruction) (uint64, bool) {
	data, err := base64.StdEncoding.DecodeString(ix.Data)
	if err != nil || len(data) < 9 {
		return 0, false
	}
	switch ix.ProgramID {
	case SystemProgram:
		if binary.LittleEndian.Uint32(data[:4]) == 2 {
			return binary.LittleEndian.Uint64(data[4:12]), true
		}
	case TokenProgram, TokenProgram2022:
		if data[0] == 3 {
			return binary.LittleEndian.Uint64(data[1:9]), true
		}
	}
	return 0, false
}

type reader struct {
	buf []byte
	pos int
	err error
}

func (r *reader) peek() byte {
	if r.err != nil || r.pos >= len(r.buf) {
		return 0
	}
	return r.buf[r.pos]
}

func (r *reader) uint8() byte {
	if r.err != nil {
		return 0
	}
	if r.pos >= len(r.buf) {
		r.err = errors.New("read past end")
		return 0
	}
	v := r.buf[r.pos]
	r.pos++
	return v
}

func (r *reader) bytes(n int) []byte {
	if r.err != nil {
		return nil
	}
	if r.pos+n > len(r.buf) {
		r.err = errors.New("read past end")
		return nil
	}
	out := r.buf[r.pos : r.pos+n]
	r.pos += n
	return out
}

func (r *reader) compactU16() int {
	v := 0
	for i := 0; i < 3; i++ {
		b := r.uint8()
		if r.err != nil {
			return 0
		}
		v |= int(b&0x7F) << (7 * i)
		if b&0x80 == 0 {
			return v
		}
	}
	return v
}

func DecodeBase64Data(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(s)
}
