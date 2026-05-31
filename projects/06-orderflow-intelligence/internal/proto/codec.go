package proto

import (
	"fmt"

	"google.golang.org/grpc/encoding"
)

const CodecName = "orderflow-proto"

type Codec struct{}

type Marshaler interface {
	MarshalProto() ([]byte, error)
}

type Unmarshaler interface {
	UnmarshalProto([]byte) error
}

func (Codec) Name() string { return CodecName }

func (Codec) Marshal(v any) ([]byte, error) {
	m, ok := v.(Marshaler)
	if !ok {
		return nil, fmt.Errorf("codec: %T does not implement MarshalProto", v)
	}
	return m.MarshalProto()
}

func (Codec) Unmarshal(data []byte, v any) error {
	u, ok := v.(Unmarshaler)
	if !ok {
		return fmt.Errorf("codec: %T does not implement UnmarshalProto", v)
	}
	return u.UnmarshalProto(data)
}

func init() {
	encoding.RegisterCodec(Codec{})
}
