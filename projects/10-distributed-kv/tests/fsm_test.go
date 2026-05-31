package tests

import (
	"bytes"
	"sync"
	"testing"

	hraft "github.com/hashicorp/raft"

	"github.com/andrey/distributed-kv/internal/store"
)

func applyCmd(t *testing.T, fsm *store.KVStore, cmd store.Command) {
	t.Helper()
	data, err := cmd.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if resp := fsm.Apply(&hraft.Log{Data: data}); resp != nil {
		if err, ok := resp.(error); ok {
			t.Fatalf("apply returned error: %v", err)
		}
	}
}

func TestApplySetThenGet(t *testing.T) {
	fsm := store.NewKVStore()
	applyCmd(t, fsm, store.Command{Op: store.OpSet, Key: "alpha", Value: []byte("one")})

	value, ok := fsm.Get("alpha")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if string(value) != "one" {
		t.Fatalf("got %q want one", value)
	}
}

func TestApplyDeleteRemovesKey(t *testing.T) {
	fsm := store.NewKVStore()
	applyCmd(t, fsm, store.Command{Op: store.OpSet, Key: "beta", Value: []byte("x")})
	applyCmd(t, fsm, store.Command{Op: store.OpDelete, Key: "beta"})

	if _, ok := fsm.Get("beta"); ok {
		t.Fatal("expected key to be deleted")
	}
}

func TestSnapshotRestoreRoundTrip(t *testing.T) {
	fsm := store.NewKVStore()
	applyCmd(t, fsm, store.Command{Op: store.OpSet, Key: "k1", Value: []byte("v1")})
	applyCmd(t, fsm, store.Command{Op: store.OpSet, Key: "k2", Value: []byte("v2")})

	snap, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	sink := newMemorySink()
	if err := snap.Persist(sink); err != nil {
		t.Fatalf("persist: %v", err)
	}

	restored := store.NewKVStore()
	if err := restored.Restore(sink.reader()); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if restored.Count() != 2 {
		t.Fatalf("got %d keys want 2", restored.Count())
	}
	v, ok := restored.Get("k2")
	if !ok || string(v) != "v2" {
		t.Fatalf("restored value mismatch: %q ok=%v", v, ok)
	}
}

func TestConcurrentReadsNoDeadlock(t *testing.T) {
	fsm := store.NewKVStore()
	applyCmd(t, fsm, store.Command{Op: store.OpSet, Key: "shared", Value: []byte("data")})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				fsm.Get("shared")
				fsm.Keys("")
				fsm.Count()
			}
		}()
	}
	wg.Wait()
}

type memorySink struct {
	buf *bytes.Buffer
}

func newMemorySink() *memorySink {
	return &memorySink{buf: new(bytes.Buffer)}
}

func (m *memorySink) Write(p []byte) (int, error) { return m.buf.Write(p) }
func (m *memorySink) Close() error                { return nil }
func (m *memorySink) ID() string                  { return "memory" }
func (m *memorySink) Cancel() error               { return nil }

func (m *memorySink) reader() *memoryReader {
	return &memoryReader{r: bytes.NewReader(m.buf.Bytes())}
}

type memoryReader struct {
	r *bytes.Reader
}

func (m *memoryReader) Read(p []byte) (int, error) { return m.r.Read(p) }
func (m *memoryReader) Close() error               { return nil }
