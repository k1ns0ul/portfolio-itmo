package grpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protowire"
)

type Message interface {
	MarshalMsg() ([]byte, error)
	UnmarshalMsg([]byte) error
}

type ForwardRequest struct {
	Method string
	Path   string
	Body   []byte
}

func (m *ForwardRequest) MarshalMsg() ([]byte, error) {
	var b []byte
	b = appendString(b, 1, m.Method)
	b = appendString(b, 2, m.Path)
	b = appendBytes(b, 3, m.Body)
	return b, nil
}

func (m *ForwardRequest) UnmarshalMsg(b []byte) error {
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 1:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.Method = string(v)
			b = b[n:]
		case 2:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.Path = string(v)
			b = b[n:]
		case 3:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.Body = append([]byte(nil), v...)
			b = b[n:]
		default:
			n := protowire.ConsumeFieldValue(num, typ, b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			b = b[n:]
		}
	}
	return nil
}

type ForwardResponse struct {
	StatusCode int32
	Body       []byte
}

func (m *ForwardResponse) MarshalMsg() ([]byte, error) {
	var b []byte
	b = appendVarint(b, 1, uint64(uint32(m.StatusCode)))
	b = appendBytes(b, 2, m.Body)
	return b, nil
}

func (m *ForwardResponse) UnmarshalMsg(b []byte) error {
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 1:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.StatusCode = int32(v)
			b = b[n:]
		case 2:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.Body = append([]byte(nil), v...)
			b = b[n:]
		default:
			n := protowire.ConsumeFieldValue(num, typ, b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			b = b[n:]
		}
	}
	return nil
}

func appendString(b []byte, num protowire.Number, s string) []byte {
	if s == "" {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.BytesType)
	return protowire.AppendString(b, s)
}

func appendBytes(b []byte, num protowire.Number, v []byte) []byte {
	if len(v) == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.BytesType)
	return protowire.AppendBytes(b, v)
}

func appendVarint(b []byte, num protowire.Number, v uint64) []byte {
	if v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, v)
}

const ForwardServiceName = "dkv.ForwardService"

type ForwardServer interface {
	Forward(context.Context, *ForwardRequest) (*ForwardResponse, error)
}

var forwardDesc = grpc.ServiceDesc{
	ServiceName: ForwardServiceName,
	HandlerType: (*ForwardServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Forward",
			Handler: func(srv any, ctx context.Context, dec func(any) error, ic grpc.UnaryServerInterceptor) (any, error) {
				in := new(ForwardRequest)
				if err := dec(in); err != nil {
					return nil, err
				}
				if ic == nil {
					return srv.(ForwardServer).Forward(ctx, in)
				}
				info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + ForwardServiceName + "/Forward"}
				return ic(ctx, in, info, func(ctx context.Context, req any) (any, error) {
					return srv.(ForwardServer).Forward(ctx, req.(*ForwardRequest))
				})
			},
		},
	},
}

func RegisterForwardServer(s grpc.ServiceRegistrar, srv ForwardServer) {
	s.RegisterService(&forwardDesc, srv)
}
