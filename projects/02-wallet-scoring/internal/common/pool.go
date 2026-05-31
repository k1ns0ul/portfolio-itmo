package common

import (
	"bytes"
	"sync"
)

var bufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

func GetBuffer() *bytes.Buffer {
	b := bufPool.Get().(*bytes.Buffer)
	b.Reset()
	return b
}

func PutBuffer(b *bytes.Buffer) {
	if b == nil || b.Cap() > 1<<20 {
		return
	}
	bufPool.Put(b)
}
