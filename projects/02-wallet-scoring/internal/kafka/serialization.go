package kafka

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/andrey/wallet-scoring/internal/models"
)

var encPool = sync.Pool{New: func() any { return new(jsonEncoder) }}

type jsonEncoder struct {
	buf []byte
}

func EncodeEnvelope(env models.Envelope) ([]byte, error) {
	enc := encPool.Get().(*jsonEncoder)
	defer func() {
		enc.buf = enc.buf[:0]
		encPool.Put(enc)
	}()
	b, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("encode envelope: %w", err)
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out, nil
}

func DecodeEnvelope(data []byte) (models.Envelope, error) {
	var env models.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return env, fmt.Errorf("decode envelope: %w", err)
	}
	return env, nil
}
