package solana

import (
	"encoding/base64"
	"errors"
	"fmt"
)

type Instruction struct {
	ProgramID string   `json:"program_id"`
	Accounts  []string `json:"accounts"`
	Data      string   `json:"data"`
	RawData   []byte   `json:"-"`
}

type ParsedTransaction struct {
	Signature    string
	AccountKeys  []string
	Instructions []Instruction
	Versioned    bool
}

func ParseRawTransaction(raw []byte) (*ParsedTransaction, error) {
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

	_ = r.uint8()
	_ = r.uint8()
	_ = r.uint8()

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
	instrs := make([]Instruction, 0, instrCount)
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
			return nil, fmt.Errorf("program index out of range")
		}
		accounts := make([]string, len(accIdx))
		for j, idx := range accIdx {
			if int(idx) >= len(keys) {
				return nil, fmt.Errorf("account index out of range")
			}
			accounts[j] = keys[idx]
		}
		instrs = append(instrs, Instruction{
			ProgramID: keys[programIdx],
			Accounts:  accounts,
			Data:      base64.StdEncoding.EncodeToString(data),
			RawData:   data,
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

	return &ParsedTransaction{
		Signature:    Base58Encode(sigs[0]),
		AccountKeys:  keys,
		Instructions: instrs,
		Versioned:    versioned,
	}, nil
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
