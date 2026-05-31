package common

import (
	"fmt"
	"math"

	"google.golang.org/protobuf/encoding/protowire"
)

type ProtoBuilder struct {
	buf []byte
}

func (b *ProtoBuilder) Varint(field int, v uint64) {
	b.buf = protowire.AppendTag(b.buf, protowire.Number(field), protowire.VarintType)
	b.buf = protowire.AppendVarint(b.buf, v)
}

func (b *ProtoBuilder) Bool(field int, v bool) {
	var x uint64
	if v {
		x = 1
	}
	b.Varint(field, x)
}

func (b *ProtoBuilder) String(field int, v string) {
	if v == "" {
		return
	}
	b.buf = protowire.AppendTag(b.buf, protowire.Number(field), protowire.BytesType)
	b.buf = protowire.AppendVarint(b.buf, uint64(len(v)))
	b.buf = append(b.buf, v...)
}

func (b *ProtoBuilder) Bytes(field int, v []byte) {
	b.buf = protowire.AppendTag(b.buf, protowire.Number(field), protowire.BytesType)
	b.buf = protowire.AppendVarint(b.buf, uint64(len(v)))
	b.buf = append(b.buf, v...)
}

func (b *ProtoBuilder) Sub(field int, sub []byte) { b.Bytes(field, sub) }

func (b *ProtoBuilder) Fixed32(field int, v uint32) {
	b.buf = protowire.AppendTag(b.buf, protowire.Number(field), protowire.Fixed32Type)
	b.buf = protowire.AppendFixed32(b.buf, v)
}

func (b *ProtoBuilder) Fixed64(field int, v uint64) {
	b.buf = protowire.AppendTag(b.buf, protowire.Number(field), protowire.Fixed64Type)
	b.buf = protowire.AppendFixed64(b.buf, v)
}

func (b *ProtoBuilder) Float(field int, v float32) { b.Fixed32(field, math.Float32bits(v)) }
func (b *ProtoBuilder) Double(field int, v float64) { b.Fixed64(field, math.Float64bits(v)) }

func (b *ProtoBuilder) Build() []byte { return b.buf }

type ProtoParser struct {
	data []byte
	Err  error
}

func NewProtoParser(data []byte) *ProtoParser { return &ProtoParser{data: data} }

func (p *ProtoParser) More() bool { return p.Err == nil && len(p.data) > 0 }

func (p *ProtoParser) Tag() (protowire.Number, protowire.Type) {
	if p.Err != nil {
		return 0, 0
	}
	num, typ, n := protowire.ConsumeTag(p.data)
	if n < 0 {
		p.Err = protowire.ParseError(n)
		return 0, 0
	}
	p.data = p.data[n:]
	return num, typ
}

func (p *ProtoParser) Varint() uint64 {
	if p.Err != nil {
		return 0
	}
	v, n := protowire.ConsumeVarint(p.data)
	if n < 0 {
		p.Err = protowire.ParseError(n)
		return 0
	}
	p.data = p.data[n:]
	return v
}

func (p *ProtoParser) Bytes() []byte {
	if p.Err != nil {
		return nil
	}
	b, n := protowire.ConsumeBytes(p.data)
	if n < 0 {
		p.Err = protowire.ParseError(n)
		return nil
	}
	p.data = p.data[n:]
	return append([]byte(nil), b...)
}

func (p *ProtoParser) String() string { return string(p.Bytes()) }

func (p *ProtoParser) Fixed32() uint32 {
	if p.Err != nil {
		return 0
	}
	v, n := protowire.ConsumeFixed32(p.data)
	if n < 0 {
		p.Err = protowire.ParseError(n)
		return 0
	}
	p.data = p.data[n:]
	return v
}

func (p *ProtoParser) Fixed64() uint64 {
	if p.Err != nil {
		return 0
	}
	v, n := protowire.ConsumeFixed64(p.data)
	if n < 0 {
		p.Err = protowire.ParseError(n)
		return 0
	}
	p.data = p.data[n:]
	return v
}

func (p *ProtoParser) Float() float32  { return math.Float32frombits(p.Fixed32()) }
func (p *ProtoParser) Double() float64 { return math.Float64frombits(p.Fixed64()) }

func (p *ProtoParser) Skip(num protowire.Number, typ protowire.Type) {
	if p.Err != nil {
		return
	}
	n := protowire.ConsumeFieldValue(num, typ, p.data)
	if n < 0 {
		p.Err = protowire.ParseError(n)
		return
	}
	p.data = p.data[n:]
}

func (p *ProtoParser) PackedVarints(dst []uint64, typ protowire.Type) []uint64 {
	if p.Err != nil {
		return dst
	}
	if typ == protowire.VarintType {
		return append(dst, p.Varint())
	}
	if typ != protowire.BytesType {
		p.Err = fmt.Errorf("unexpected wire type %d for packed varint", typ)
		return dst
	}
	chunk := p.Bytes()
	for len(chunk) > 0 {
		v, n := protowire.ConsumeVarint(chunk)
		if n < 0 {
			p.Err = protowire.ParseError(n)
			return dst
		}
		dst = append(dst, v)
		chunk = chunk[n:]
	}
	return dst
}
