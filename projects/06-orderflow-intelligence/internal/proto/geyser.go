package proto

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"
)

type CommitmentLevel int32

const (
	Processed CommitmentLevel = 0
	Confirmed CommitmentLevel = 1
	Finalized CommitmentLevel = 2
)

func CommitmentFromString(s string) CommitmentLevel {
	switch s {
	case "processed":
		return Processed
	case "finalized":
		return Finalized
	default:
		return Confirmed
	}
}

type SubscribeRequest struct {
	Transactions map[string]*SubscribeRequestFilterTransactions
	Commitment   CommitmentLevel
}

type SubscribeRequestFilterTransactions struct {
	Vote            *bool
	Failed          *bool
	AccountInclude  []string
	AccountExclude  []string
	AccountRequired []string
}

type SubscribeUpdate struct {
	Filters     []string
	Transaction *SubscribeUpdateTransaction
	Ping        *SubscribeUpdatePing
}

type SubscribeUpdatePing struct{}

type SubscribeUpdateTransaction struct {
	Transaction *TransactionInfo
	Slot        uint64
}

type TransactionInfo struct {
	Signature   []byte
	IsVote      bool
	Transaction *Transaction
	Meta        *TransactionStatusMeta
	Index       uint64
}

type Transaction struct {
	Signatures [][]byte
	Message    *Message
}

type Message struct {
	Header          *MessageHeader
	AccountKeys     [][]byte
	RecentBlockhash []byte
	Instructions    []*CompiledInstruction
}

type MessageHeader struct {
	NumRequiredSignatures       uint32
	NumReadonlySignedAccounts   uint32
	NumReadonlyUnsignedAccounts uint32
}

type CompiledInstruction struct {
	ProgramIDIndex uint32
	Accounts       []byte
	Data           []byte
}

type TransactionStatusMeta struct {
	Err []byte
	Fee uint64
}

func (r *SubscribeRequest) MarshalProto() ([]byte, error) {
	var b builder
	for k, v := range r.Transactions {
		vb, err := v.MarshalProto()
		if err != nil {
			return nil, err
		}
		var entry builder
		entry.appendString(1, k)
		entry.appendBytes(2, vb)
		b.appendBytes(3, entry.buf)
	}
	b.appendVarint(6, uint64(r.Commitment))
	return b.buf, nil
}

func (f *SubscribeRequestFilterTransactions) MarshalProto() ([]byte, error) {
	var b builder
	if f.Vote != nil {
		b.appendBool(1, *f.Vote)
	}
	if f.Failed != nil {
		b.appendBool(2, *f.Failed)
	}
	for _, s := range f.AccountInclude {
		b.appendString(4, s)
	}
	for _, s := range f.AccountExclude {
		b.appendString(5, s)
	}
	for _, s := range f.AccountRequired {
		b.appendString(6, s)
	}
	return b.buf, nil
}

func (u *SubscribeUpdate) UnmarshalProto(data []byte) error {
	p := &parser{data: data}
	for p.more() {
		num, typ := p.tag()
		switch num {
		case 1:
			u.Filters = append(u.Filters, p.string())
		case 4:
			u.Transaction = &SubscribeUpdateTransaction{}
			if err := u.Transaction.UnmarshalProto(p.bytes()); err != nil {
				return err
			}
		case 6:
			u.Ping = &SubscribeUpdatePing{}
			_ = p.bytes()
		default:
			p.skip(num, typ)
		}
	}
	return p.err
}

func (t *SubscribeUpdateTransaction) UnmarshalProto(data []byte) error {
	p := &parser{data: data}
	for p.more() {
		num, typ := p.tag()
		switch num {
		case 1:
			t.Transaction = &TransactionInfo{}
			if err := t.Transaction.UnmarshalProto(p.bytes()); err != nil {
				return err
			}
		case 2:
			t.Slot = p.varint()
		default:
			p.skip(num, typ)
		}
	}
	return p.err
}

func (t *TransactionInfo) UnmarshalProto(data []byte) error {
	p := &parser{data: data}
	for p.more() {
		num, typ := p.tag()
		switch num {
		case 1:
			t.Signature = p.bytes()
		case 2:
			t.IsVote = p.varint() != 0
		case 3:
			t.Transaction = &Transaction{}
			if err := t.Transaction.UnmarshalProto(p.bytes()); err != nil {
				return err
			}
		case 4:
			t.Meta = &TransactionStatusMeta{}
			if err := t.Meta.UnmarshalProto(p.bytes()); err != nil {
				return err
			}
		case 5:
			t.Index = p.varint()
		default:
			p.skip(num, typ)
		}
	}
	return p.err
}

func (t *Transaction) UnmarshalProto(data []byte) error {
	p := &parser{data: data}
	for p.more() {
		num, typ := p.tag()
		switch num {
		case 1:
			t.Signatures = append(t.Signatures, p.bytes())
		case 2:
			t.Message = &Message{}
			if err := t.Message.UnmarshalProto(p.bytes()); err != nil {
				return err
			}
		default:
			p.skip(num, typ)
		}
	}
	return p.err
}

func (m *Message) UnmarshalProto(data []byte) error {
	p := &parser{data: data}
	for p.more() {
		num, typ := p.tag()
		switch num {
		case 1:
			m.Header = &MessageHeader{}
			if err := m.Header.UnmarshalProto(p.bytes()); err != nil {
				return err
			}
		case 2:
			m.AccountKeys = append(m.AccountKeys, p.bytes())
		case 3:
			m.RecentBlockhash = p.bytes()
		case 4:
			ix := &CompiledInstruction{}
			if err := ix.UnmarshalProto(p.bytes()); err != nil {
				return err
			}
			m.Instructions = append(m.Instructions, ix)
		default:
			p.skip(num, typ)
		}
	}
	return p.err
}

func (h *MessageHeader) UnmarshalProto(data []byte) error {
	p := &parser{data: data}
	for p.more() {
		num, typ := p.tag()
		switch num {
		case 1:
			h.NumRequiredSignatures = uint32(p.varint())
		case 2:
			h.NumReadonlySignedAccounts = uint32(p.varint())
		case 3:
			h.NumReadonlyUnsignedAccounts = uint32(p.varint())
		default:
			p.skip(num, typ)
		}
	}
	return p.err
}

func (c *CompiledInstruction) UnmarshalProto(data []byte) error {
	p := &parser{data: data}
	for p.more() {
		num, typ := p.tag()
		switch num {
		case 1:
			c.ProgramIDIndex = uint32(p.varint())
		case 2:
			c.Accounts = p.bytes()
		case 3:
			c.Data = p.bytes()
		default:
			p.skip(num, typ)
		}
	}
	return p.err
}

func (m *TransactionStatusMeta) UnmarshalProto(data []byte) error {
	p := &parser{data: data}
	for p.more() {
		num, typ := p.tag()
		switch num {
		case 1:
			m.Err = p.bytes()
		case 2:
			m.Fee = p.varint()
		default:
			p.skip(num, typ)
		}
	}
	return p.err
}

type builder struct {
	buf []byte
}

func (b *builder) appendTag(field int, typ protowire.Type) {
	b.buf = protowire.AppendTag(b.buf, protowire.Number(field), typ)
}

func (b *builder) appendVarint(field int, v uint64) {
	b.appendTag(field, protowire.VarintType)
	b.buf = protowire.AppendVarint(b.buf, v)
}

func (b *builder) appendBool(field int, v bool) {
	var x uint64
	if v {
		x = 1
	}
	b.appendVarint(field, x)
}

func (b *builder) appendString(field int, v string) {
	b.appendTag(field, protowire.BytesType)
	b.buf = protowire.AppendVarint(b.buf, uint64(len(v)))
	b.buf = append(b.buf, v...)
}

func (b *builder) appendBytes(field int, v []byte) {
	b.appendTag(field, protowire.BytesType)
	b.buf = protowire.AppendVarint(b.buf, uint64(len(v)))
	b.buf = append(b.buf, v...)
}

type parser struct {
	data []byte
	err  error
}

func (p *parser) more() bool { return p.err == nil && len(p.data) > 0 }

func (p *parser) tag() (protowire.Number, protowire.Type) {
	if p.err != nil {
		return 0, 0
	}
	num, typ, n := protowire.ConsumeTag(p.data)
	if n < 0 {
		p.err = fmt.Errorf("consume tag: %w", protowire.ParseError(n))
		return 0, 0
	}
	p.data = p.data[n:]
	return num, typ
}

func (p *parser) varint() uint64 {
	if p.err != nil {
		return 0
	}
	v, n := protowire.ConsumeVarint(p.data)
	if n < 0 {
		p.err = protowire.ParseError(n)
		return 0
	}
	p.data = p.data[n:]
	return v
}

func (p *parser) bytes() []byte {
	if p.err != nil {
		return nil
	}
	b, n := protowire.ConsumeBytes(p.data)
	if n < 0 {
		p.err = protowire.ParseError(n)
		return nil
	}
	p.data = p.data[n:]
	return append([]byte(nil), b...)
}

func (p *parser) string() string { return string(p.bytes()) }

func (p *parser) skip(num protowire.Number, typ protowire.Type) {
	if p.err != nil {
		return
	}
	n := protowire.ConsumeFieldValue(num, typ, p.data)
	if n < 0 {
		p.err = protowire.ParseError(n)
		return
	}
	p.data = p.data[n:]
}
