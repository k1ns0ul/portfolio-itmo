package grpc

import (
	"fmt"

	"google.golang.org/grpc/encoding"
)

const codecName = "dkv"

type dkvCodec struct{}

func (dkvCodec) Marshal(v any) ([]byte, error) {
	msg, ok := v.(Message)
	if !ok {
		return nil, fmt.Errorf("codec marshal: %T is not a Message", v)
	}
	return msg.MarshalMsg()
}

func (dkvCodec) Unmarshal(data []byte, v any) error {
	msg, ok := v.(Message)
	if !ok {
		return fmt.Errorf("codec unmarshal: %T is not a Message", v)
	}
	return msg.UnmarshalMsg(data)
}

func (dkvCodec) Name() string { return codecName }

func init() {
	encoding.RegisterCodec(dkvCodec{})
}
