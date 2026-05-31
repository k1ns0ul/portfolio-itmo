package grpc

import (
	"fmt"

	"google.golang.org/grpc/encoding"
)

const codecName = "proto"

type mpcCodec struct{}

func (mpcCodec) Marshal(v any) ([]byte, error) {
	msg, ok := v.(Message)
	if !ok {
		return nil, fmt.Errorf("codec: %T does not implement Message", v)
	}
	return msg.MarshalMPC()
}

func (mpcCodec) Unmarshal(data []byte, v any) error {
	msg, ok := v.(Message)
	if !ok {
		return fmt.Errorf("codec: %T does not implement Message", v)
	}
	return msg.UnmarshalMPC(data)
}

func (mpcCodec) Name() string { return codecName }

func init() {
	encoding.RegisterCodec(mpcCodec{})
}
