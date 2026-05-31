package pb

import "github.com/andrey/wallet-scoring/internal/common"

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
	Versioned       bool
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
	Err          []byte
	Fee          uint64
	PreBalances  []uint64
	PostBalances []uint64
}

func (r *SubscribeRequest) MarshalProto() ([]byte, error) {
	var b common.ProtoBuilder
	for k, v := range r.Transactions {
		vb, err := v.MarshalProto()
		if err != nil {
			return nil, err
		}
		var entry common.ProtoBuilder
		entry.String(1, k)
		entry.Bytes(2, vb)
		b.Bytes(3, entry.Build())
	}
	b.Varint(6, uint64(r.Commitment))
	return b.Build(), nil
}

func (f *SubscribeRequestFilterTransactions) MarshalProto() ([]byte, error) {
	var b common.ProtoBuilder
	if f.Vote != nil {
		b.Bool(1, *f.Vote)
	}
	if f.Failed != nil {
		b.Bool(2, *f.Failed)
	}
	for _, s := range f.AccountInclude {
		b.String(4, s)
	}
	for _, s := range f.AccountExclude {
		b.String(5, s)
	}
	for _, s := range f.AccountRequired {
		b.String(6, s)
	}
	return b.Build(), nil
}

func (u *SubscribeUpdate) UnmarshalProto(data []byte) error {
	p := common.NewProtoParser(data)
	for p.More() {
		num, typ := p.Tag()
		switch num {
		case 1:
			u.Filters = append(u.Filters, p.String())
		case 4:
			u.Transaction = &SubscribeUpdateTransaction{}
			if err := u.Transaction.UnmarshalProto(p.Bytes()); err != nil {
				return err
			}
		case 6:
			u.Ping = &SubscribeUpdatePing{}
			_ = p.Bytes()
		default:
			p.Skip(num, typ)
		}
	}
	return p.Err
}

func (t *SubscribeUpdateTransaction) UnmarshalProto(data []byte) error {
	p := common.NewProtoParser(data)
	for p.More() {
		num, typ := p.Tag()
		switch num {
		case 1:
			t.Transaction = &TransactionInfo{}
			if err := t.Transaction.UnmarshalProto(p.Bytes()); err != nil {
				return err
			}
		case 2:
			t.Slot = p.Varint()
		default:
			p.Skip(num, typ)
		}
	}
	return p.Err
}

func (t *TransactionInfo) UnmarshalProto(data []byte) error {
	p := common.NewProtoParser(data)
	for p.More() {
		num, typ := p.Tag()
		switch num {
		case 1:
			t.Signature = p.Bytes()
		case 2:
			t.IsVote = p.Varint() != 0
		case 3:
			t.Transaction = &Transaction{}
			if err := t.Transaction.UnmarshalProto(p.Bytes()); err != nil {
				return err
			}
		case 4:
			t.Meta = &TransactionStatusMeta{}
			if err := t.Meta.UnmarshalProto(p.Bytes()); err != nil {
				return err
			}
		case 5:
			t.Index = p.Varint()
		default:
			p.Skip(num, typ)
		}
	}
	return p.Err
}

func (t *Transaction) UnmarshalProto(data []byte) error {
	p := common.NewProtoParser(data)
	for p.More() {
		num, typ := p.Tag()
		switch num {
		case 1:
			t.Signatures = append(t.Signatures, p.Bytes())
		case 2:
			t.Message = &Message{}
			if err := t.Message.UnmarshalProto(p.Bytes()); err != nil {
				return err
			}
		default:
			p.Skip(num, typ)
		}
	}
	return p.Err
}

func (m *Message) UnmarshalProto(data []byte) error {
	p := common.NewProtoParser(data)
	for p.More() {
		num, typ := p.Tag()
		switch num {
		case 1:
			m.Header = &MessageHeader{}
			if err := m.Header.UnmarshalProto(p.Bytes()); err != nil {
				return err
			}
		case 2:
			m.AccountKeys = append(m.AccountKeys, p.Bytes())
		case 3:
			m.RecentBlockhash = p.Bytes()
		case 4:
			ix := &CompiledInstruction{}
			if err := ix.UnmarshalProto(p.Bytes()); err != nil {
				return err
			}
			m.Instructions = append(m.Instructions, ix)
		case 5:
			m.Versioned = p.Varint() != 0
		default:
			p.Skip(num, typ)
		}
	}
	return p.Err
}

func (h *MessageHeader) UnmarshalProto(data []byte) error {
	p := common.NewProtoParser(data)
	for p.More() {
		num, typ := p.Tag()
		switch num {
		case 1:
			h.NumRequiredSignatures = uint32(p.Varint())
		case 2:
			h.NumReadonlySignedAccounts = uint32(p.Varint())
		case 3:
			h.NumReadonlyUnsignedAccounts = uint32(p.Varint())
		default:
			p.Skip(num, typ)
		}
	}
	return p.Err
}

func (c *CompiledInstruction) UnmarshalProto(data []byte) error {
	p := common.NewProtoParser(data)
	for p.More() {
		num, typ := p.Tag()
		switch num {
		case 1:
			c.ProgramIDIndex = uint32(p.Varint())
		case 2:
			c.Accounts = p.Bytes()
		case 3:
			c.Data = p.Bytes()
		default:
			p.Skip(num, typ)
		}
	}
	return p.Err
}

func (m *TransactionStatusMeta) UnmarshalProto(data []byte) error {
	p := common.NewProtoParser(data)
	for p.More() {
		num, typ := p.Tag()
		switch num {
		case 1:
			m.Err = p.Bytes()
		case 2:
			m.Fee = p.Varint()
		case 3:
			m.PreBalances = p.PackedVarints(m.PreBalances, typ)
		case 4:
			m.PostBalances = p.PackedVarints(m.PostBalances, typ)
		default:
			p.Skip(num, typ)
		}
	}
	return p.Err
}
