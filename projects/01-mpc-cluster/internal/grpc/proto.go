package grpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protowire"
)

type Message interface {
	MarshalMPC() ([]byte, error)
	UnmarshalMPC([]byte) error
}

type NodeInfo struct {
	SessionID string
	NodeID    int32
	Addr      string
}

func (m *NodeInfo) MarshalMPC() ([]byte, error) {
	var b []byte
	b = appendString(b, 1, m.SessionID)
	b = appendVarint(b, 2, uint64(uint32(m.NodeID)))
	b = appendString(b, 3, m.Addr)
	return b, nil
}

func (m *NodeInfo) UnmarshalMPC(b []byte) error {
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
			m.SessionID = string(v)
			b = b[n:]
		case 2:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.NodeID = int32(v)
			b = b[n:]
		case 3:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.Addr = string(v)
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

type Ack struct {
	Ok      bool
	Message string
}

func (m *Ack) MarshalMPC() ([]byte, error) {
	var b []byte
	b = appendBool(b, 1, m.Ok)
	b = appendString(b, 2, m.Message)
	return b, nil
}

func (m *Ack) UnmarshalMPC(b []byte) error {
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
			m.Ok = v != 0
			b = b[n:]
		case 2:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.Message = string(v)
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

type TaskRequest struct {
	SessionID string
	NodeID    int32
}

func (m *TaskRequest) MarshalMPC() ([]byte, error) {
	var b []byte
	b = appendString(b, 1, m.SessionID)
	b = appendVarint(b, 2, uint64(uint32(m.NodeID)))
	return b, nil
}

func (m *TaskRequest) UnmarshalMPC(b []byte) error {
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
			m.SessionID = string(v)
			b = b[n:]
		case 2:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.NodeID = int32(v)
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

type PeerEntry struct {
	NodeID int32
	Addr   string
}

func (m *PeerEntry) MarshalMPC() ([]byte, error) {
	var b []byte
	b = appendVarint(b, 1, uint64(uint32(m.NodeID)))
	b = appendString(b, 2, m.Addr)
	return b, nil
}

func (m *PeerEntry) UnmarshalMPC(b []byte) error {
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
			m.NodeID = int32(v)
			b = b[n:]
		case 2:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.Addr = string(v)
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

type ShareData struct {
	Slot  int32
	Value []byte
}

func (m *ShareData) MarshalMPC() ([]byte, error) {
	var b []byte
	b = appendVarint(b, 1, uint64(uint32(m.Slot)))
	b = appendBytes(b, 2, m.Value)
	return b, nil
}

func (m *ShareData) UnmarshalMPC(b []byte) error {
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
			m.Slot = int32(v)
			b = b[n:]
		case 2:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.Value = append([]byte(nil), v...)
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

type TripleData struct {
	A []byte
	B []byte
	C []byte
}

func (m *TripleData) MarshalMPC() ([]byte, error) {
	var b []byte
	b = appendBytes(b, 1, m.A)
	b = appendBytes(b, 2, m.B)
	b = appendBytes(b, 3, m.C)
	return b, nil
}

func (m *TripleData) UnmarshalMPC(b []byte) error {
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
			m.A = append([]byte(nil), v...)
			b = b[n:]
		case 2:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.B = append([]byte(nil), v...)
			b = b[n:]
		case 3:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.C = append([]byte(nil), v...)
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

type Task struct {
	SessionID  string
	NodeID     int32
	Op         string
	PartyCount int32
	BitLen     int32
	Ready      bool
	Peers      []*PeerEntry
	Shares     []*ShareData
	Triples    []*TripleData
}

func (m *Task) MarshalMPC() ([]byte, error) {
	var b []byte
	b = appendString(b, 1, m.SessionID)
	b = appendVarint(b, 2, uint64(uint32(m.NodeID)))
	b = appendString(b, 3, m.Op)
	b = appendVarint(b, 4, uint64(uint32(m.PartyCount)))
	b = appendVarint(b, 5, uint64(uint32(m.BitLen)))
	b = appendBool(b, 6, m.Ready)
	for _, p := range m.Peers {
		sub, err := p.MarshalMPC()
		if err != nil {
			return nil, err
		}
		b = appendBytes(b, 7, sub)
	}
	for _, s := range m.Shares {
		sub, err := s.MarshalMPC()
		if err != nil {
			return nil, err
		}
		b = appendBytes(b, 8, sub)
	}
	for _, t := range m.Triples {
		sub, err := t.MarshalMPC()
		if err != nil {
			return nil, err
		}
		b = appendBytes(b, 9, sub)
	}
	return b, nil
}

func (m *Task) UnmarshalMPC(b []byte) error {
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
			m.SessionID = string(v)
			b = b[n:]
		case 2:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.NodeID = int32(v)
			b = b[n:]
		case 3:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.Op = string(v)
			b = b[n:]
		case 4:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.PartyCount = int32(v)
			b = b[n:]
		case 5:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.BitLen = int32(v)
			b = b[n:]
		case 6:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.Ready = v != 0
			b = b[n:]
		case 7:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			pe := &PeerEntry{}
			if err := pe.UnmarshalMPC(v); err != nil {
				return err
			}
			m.Peers = append(m.Peers, pe)
			b = b[n:]
		case 8:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			sd := &ShareData{}
			if err := sd.UnmarshalMPC(v); err != nil {
				return err
			}
			m.Shares = append(m.Shares, sd)
			b = b[n:]
		case 9:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			td := &TripleData{}
			if err := td.UnmarshalMPC(v); err != nil {
				return err
			}
			m.Triples = append(m.Triples, td)
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

type ResultMsg struct {
	SessionID string
	NodeID    int32
	Value     []byte
	Err       string
}

func (m *ResultMsg) MarshalMPC() ([]byte, error) {
	var b []byte
	b = appendString(b, 1, m.SessionID)
	b = appendVarint(b, 2, uint64(uint32(m.NodeID)))
	b = appendBytes(b, 3, m.Value)
	b = appendString(b, 4, m.Err)
	return b, nil
}

func (m *ResultMsg) UnmarshalMPC(b []byte) error {
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
			m.SessionID = string(v)
			b = b[n:]
		case 2:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.NodeID = int32(v)
			b = b[n:]
		case 3:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.Value = append([]byte(nil), v...)
			b = b[n:]
		case 4:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.Err = string(v)
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

type Msg struct {
	RoundNum  int32
	SenderID  int32
	SessionID string
	Data      []byte
}

func (m *Msg) MarshalMPC() ([]byte, error) {
	var b []byte
	b = appendVarint(b, 1, uint64(uint32(m.RoundNum)))
	b = appendVarint(b, 2, uint64(uint32(m.SenderID)))
	b = appendString(b, 3, m.SessionID)
	b = appendBytes(b, 4, m.Data)
	return b, nil
}

func (m *Msg) UnmarshalMPC(b []byte) error {
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
			m.RoundNum = int32(v)
			b = b[n:]
		case 2:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.SenderID = int32(v)
			b = b[n:]
		case 3:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.SessionID = string(v)
			b = b[n:]
		case 4:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.Data = append([]byte(nil), v...)
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

type HeartbeatPing struct {
	NodeID    int32
	SessionID string
	Ts        int64
}

func (m *HeartbeatPing) MarshalMPC() ([]byte, error) {
	var b []byte
	b = appendVarint(b, 1, uint64(uint32(m.NodeID)))
	b = appendString(b, 2, m.SessionID)
	b = appendVarint(b, 3, uint64(m.Ts))
	return b, nil
}

func (m *HeartbeatPing) UnmarshalMPC(b []byte) error {
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
			m.NodeID = int32(v)
			b = b[n:]
		case 2:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.SessionID = string(v)
			b = b[n:]
		case 3:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.Ts = int64(v)
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

type HeartbeatPong struct {
	Ok      bool
	Command string
}

func (m *HeartbeatPong) MarshalMPC() ([]byte, error) {
	var b []byte
	b = appendBool(b, 1, m.Ok)
	b = appendString(b, 2, m.Command)
	return b, nil
}

func (m *HeartbeatPong) UnmarshalMPC(b []byte) error {
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
			m.Ok = v != 0
			b = b[n:]
		case 2:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			m.Command = string(v)
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

func appendBool(b []byte, num protowire.Number, v bool) []byte {
	if !v {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, 1)
}

const (
	CoordinatorServiceName = "mpc.Coordinator"
	PeerServiceName        = "mpc.Peer"
)

type CoordinatorServer interface {
	RegisterNode(context.Context, *NodeInfo) (*Ack, error)
	GetTask(context.Context, *TaskRequest) (*Task, error)
	SubmitResult(context.Context, *ResultMsg) (*Ack, error)
	Heartbeat(grpc.ServerStream) error
}

type PeerServer interface {
	Exchange(context.Context, *Msg) (*Ack, error)
}

var coordinatorDesc = grpc.ServiceDesc{
	ServiceName: CoordinatorServiceName,
	HandlerType: (*CoordinatorServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "RegisterNode",
			Handler: func(srv any, ctx context.Context, dec func(any) error, ic grpc.UnaryServerInterceptor) (any, error) {
				in := new(NodeInfo)
				if err := dec(in); err != nil {
					return nil, err
				}
				if ic == nil {
					return srv.(CoordinatorServer).RegisterNode(ctx, in)
				}
				info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + CoordinatorServiceName + "/RegisterNode"}
				return ic(ctx, in, info, func(ctx context.Context, req any) (any, error) {
					return srv.(CoordinatorServer).RegisterNode(ctx, req.(*NodeInfo))
				})
			},
		},
		{
			MethodName: "GetTask",
			Handler: func(srv any, ctx context.Context, dec func(any) error, ic grpc.UnaryServerInterceptor) (any, error) {
				in := new(TaskRequest)
				if err := dec(in); err != nil {
					return nil, err
				}
				if ic == nil {
					return srv.(CoordinatorServer).GetTask(ctx, in)
				}
				info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + CoordinatorServiceName + "/GetTask"}
				return ic(ctx, in, info, func(ctx context.Context, req any) (any, error) {
					return srv.(CoordinatorServer).GetTask(ctx, req.(*TaskRequest))
				})
			},
		},
		{
			MethodName: "SubmitResult",
			Handler: func(srv any, ctx context.Context, dec func(any) error, ic grpc.UnaryServerInterceptor) (any, error) {
				in := new(ResultMsg)
				if err := dec(in); err != nil {
					return nil, err
				}
				if ic == nil {
					return srv.(CoordinatorServer).SubmitResult(ctx, in)
				}
				info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + CoordinatorServiceName + "/SubmitResult"}
				return ic(ctx, in, info, func(ctx context.Context, req any) (any, error) {
					return srv.(CoordinatorServer).SubmitResult(ctx, req.(*ResultMsg))
				})
			},
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName: "Heartbeat",
			Handler: func(srv any, stream grpc.ServerStream) error {
				return srv.(CoordinatorServer).Heartbeat(stream)
			},
			ServerStreams: true,
			ClientStreams: true,
		},
	},
}

var peerDesc = grpc.ServiceDesc{
	ServiceName: PeerServiceName,
	HandlerType: (*PeerServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Exchange",
			Handler: func(srv any, ctx context.Context, dec func(any) error, ic grpc.UnaryServerInterceptor) (any, error) {
				in := new(Msg)
				if err := dec(in); err != nil {
					return nil, err
				}
				if ic == nil {
					return srv.(PeerServer).Exchange(ctx, in)
				}
				info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + PeerServiceName + "/Exchange"}
				return ic(ctx, in, info, func(ctx context.Context, req any) (any, error) {
					return srv.(PeerServer).Exchange(ctx, req.(*Msg))
				})
			},
		},
	},
}

func RegisterCoordinatorServer(s grpc.ServiceRegistrar, srv CoordinatorServer) {
	s.RegisterService(&coordinatorDesc, srv)
}

func RegisterPeerServer(s grpc.ServiceRegistrar, srv PeerServer) {
	s.RegisterService(&peerDesc, srv)
}
