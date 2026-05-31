package agg

import "github.com/andrey/wallet-scoring/internal/common"

type Empty struct{}

func (Empty) MarshalProto() ([]byte, error) { return nil, nil }
func (Empty) UnmarshalProto(_ []byte) error { return nil }

type GetWalletStatsRequest struct {
	Address string
}

func (r *GetWalletStatsRequest) MarshalProto() ([]byte, error) {
	var b common.ProtoBuilder
	b.String(1, r.Address)
	return b.Build(), nil
}

func (r *GetWalletStatsRequest) UnmarshalProto(data []byte) error {
	p := common.NewProtoParser(data)
	for p.More() {
		num, typ := p.Tag()
		switch num {
		case 1:
			r.Address = p.String()
		default:
			p.Skip(num, typ)
		}
	}
	return p.Err
}

type WalletStatsResponse struct {
	Wallet               string
	TxCount              uint64
	FirstSeenUnix        int64
	LastSeenUnix         int64
	UniqueCounterparties uint32
	AvgTxAmount          float64
	MedianTxAmount       float64
	HerfindahlIndex      float64
	SmartContractRatio   float32
	VelocityPerHour      float64
	DormancyDays         float64
	Score                float32
	Category             string
	UpdatedAtUnix        int64
	Found                bool
}

func (r *WalletStatsResponse) MarshalProto() ([]byte, error) {
	var b common.ProtoBuilder
	b.String(1, r.Wallet)
	b.Varint(2, r.TxCount)
	b.Varint(3, uint64(r.FirstSeenUnix))
	b.Varint(4, uint64(r.LastSeenUnix))
	b.Varint(5, uint64(r.UniqueCounterparties))
	b.Double(6, r.AvgTxAmount)
	b.Double(7, r.MedianTxAmount)
	b.Double(8, r.HerfindahlIndex)
	b.Float(9, r.SmartContractRatio)
	b.Double(10, r.VelocityPerHour)
	b.Double(11, r.DormancyDays)
	b.Float(12, r.Score)
	b.String(13, r.Category)
	b.Varint(14, uint64(r.UpdatedAtUnix))
	b.Bool(15, r.Found)
	return b.Build(), nil
}

func (r *WalletStatsResponse) UnmarshalProto(data []byte) error {
	p := common.NewProtoParser(data)
	for p.More() {
		num, typ := p.Tag()
		switch num {
		case 1:
			r.Wallet = p.String()
		case 2:
			r.TxCount = p.Varint()
		case 3:
			r.FirstSeenUnix = int64(p.Varint())
		case 4:
			r.LastSeenUnix = int64(p.Varint())
		case 5:
			r.UniqueCounterparties = uint32(p.Varint())
		case 6:
			r.AvgTxAmount = p.Double()
		case 7:
			r.MedianTxAmount = p.Double()
		case 8:
			r.HerfindahlIndex = p.Double()
		case 9:
			r.SmartContractRatio = p.Float()
		case 10:
			r.VelocityPerHour = p.Double()
		case 11:
			r.DormancyDays = p.Double()
		case 12:
			r.Score = p.Float()
		case 13:
			r.Category = p.String()
		case 14:
			r.UpdatedAtUnix = int64(p.Varint())
		case 15:
			r.Found = p.Varint() != 0
		default:
			p.Skip(num, typ)
		}
	}
	return p.Err
}

type GetTopWalletsRequest struct {
	Limit  uint32
	Offset uint32
}

func (r *GetTopWalletsRequest) MarshalProto() ([]byte, error) {
	var b common.ProtoBuilder
	b.Varint(1, uint64(r.Limit))
	b.Varint(2, uint64(r.Offset))
	return b.Build(), nil
}

func (r *GetTopWalletsRequest) UnmarshalProto(data []byte) error {
	p := common.NewProtoParser(data)
	for p.More() {
		num, typ := p.Tag()
		switch num {
		case 1:
			r.Limit = uint32(p.Varint())
		case 2:
			r.Offset = uint32(p.Varint())
		default:
			p.Skip(num, typ)
		}
	}
	return p.Err
}

type TopWalletsResponse struct {
	Items []*WalletStatsResponse
}

func (r *TopWalletsResponse) MarshalProto() ([]byte, error) {
	var b common.ProtoBuilder
	for _, it := range r.Items {
		sub, err := it.MarshalProto()
		if err != nil {
			return nil, err
		}
		b.Sub(1, sub)
	}
	return b.Build(), nil
}

func (r *TopWalletsResponse) UnmarshalProto(data []byte) error {
	p := common.NewProtoParser(data)
	for p.More() {
		num, typ := p.Tag()
		switch num {
		case 1:
			it := &WalletStatsResponse{}
			if err := it.UnmarshalProto(p.Bytes()); err != nil {
				return err
			}
			r.Items = append(r.Items, it)
		default:
			p.Skip(num, typ)
		}
	}
	return p.Err
}

type RefreshWalletRequest struct {
	Address string
}

func (r *RefreshWalletRequest) MarshalProto() ([]byte, error) {
	var b common.ProtoBuilder
	b.String(1, r.Address)
	return b.Build(), nil
}

func (r *RefreshWalletRequest) UnmarshalProto(data []byte) error {
	p := common.NewProtoParser(data)
	for p.More() {
		num, typ := p.Tag()
		switch num {
		case 1:
			r.Address = p.String()
		default:
			p.Skip(num, typ)
		}
	}
	return p.Err
}
