package telegram

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestStreamChunkSizeUsesOneMiBForLargeRangesAndShrinksForSmallRanges(t *testing.T) {
	if got := streamChunkSize(0, 4*1024*1024); got != 1024*1024 {
		t.Fatalf("large chunk size = %d, want 1048576", got)
	}
	if got := streamChunkSize(0, 16*1024); got != 16*1024 {
		t.Fatalf("small chunk size = %d, want 16384", got)
	}
	if got := streamChunkSize(0, 700); got != 1024 {
		t.Fatalf("minimum chunk size = %d, want 1024", got)
	}
}

func TestRangePrefetchReaderClipsUnalignedRange(t *testing.T) {
	src := newTestChunkSource(4, 1024)
	reader := newRangePrefetchReader(context.Background(), 100, 2300, StreamConfig{
		Concurrency:  2,
		Buffers:      2,
		ChunkTimeout: time.Second,
	}, src)
	defer reader.Close()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}

	want := src.bytes[100 : 2300+1]
	if !bytes.Equal(got, want) {
		t.Fatalf("read bytes mismatch: got %d bytes, want %d bytes", len(got), len(want))
	}
}

func TestRangePrefetchReaderPreservesOrderWhenChunksCompleteOutOfOrder(t *testing.T) {
	src := newTestChunkSource(4, 1024)
	delayed := &delayedChunkSource{testChunkSource: src, delays: map[int64]time.Duration{0: 40 * time.Millisecond}}
	reader := newRangePrefetchReader(context.Background(), 0, 4095, StreamConfig{
		Concurrency:  2,
		Buffers:      2,
		ChunkTimeout: time.Second,
	}, delayed)
	defer reader.Close()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if !bytes.Equal(got, src.bytes) {
		t.Fatal("reader returned chunks out of order")
	}
	if !slices.Equal(delayed.offsets(), []int64{0, 1024, 2048, 3072}) {
		t.Fatalf("chunk offsets = %v", delayed.offsets())
	}
}

func TestRangePrefetchReaderCloseCancelsChunkFetch(t *testing.T) {
	src := &blockingChunkSource{started: make(chan struct{}), canceled: make(chan struct{})}
	reader := newRangePrefetchReader(context.Background(), 0, 2047, StreamConfig{
		Concurrency:  1,
		Buffers:      1,
		ChunkTimeout: time.Minute,
	}, src)

	<-src.started
	if err := reader.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	select {
	case <-src.canceled:
	case <-time.After(time.Second):
		t.Fatal("chunk context was not canceled")
	}
}

func TestRangePrefetchReaderReportsChunkTimeout(t *testing.T) {
	src := &sleepingChunkSource{sleep: 100 * time.Millisecond}
	reader := newRangePrefetchReader(context.Background(), 0, 2047, StreamConfig{
		Concurrency:  1,
		Buffers:      1,
		ChunkTimeout: 10 * time.Millisecond,
	}, src)
	defer reader.Close()

	_, err := io.ReadAll(reader)
	if !errors.Is(err, ErrChunkTimeout) {
		t.Fatalf("ReadAll error = %v, want ErrChunkTimeout", err)
	}
}

type testChunkSource struct {
	bytes     []byte
	chunkSize int64
}

func newTestChunkSource(chunks int, chunkSize int64) *testChunkSource {
	data := make([]byte, chunks*int(chunkSize))
	for i := range data {
		data[i] = byte(i % 251)
	}
	return &testChunkSource{bytes: data, chunkSize: chunkSize}
}

func (s *testChunkSource) ChunkSize(start, end int64) int64 {
	return s.chunkSize
}

func (s *testChunkSource) Chunk(ctx context.Context, offset int64, limit int64) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	end := offset + limit
	if end > int64(len(s.bytes)) {
		end = int64(len(s.bytes))
	}
	return append([]byte(nil), s.bytes[offset:end]...), nil
}

type delayedChunkSource struct {
	*testChunkSource
	mu     sync.Mutex
	delays map[int64]time.Duration
	seen   []int64
}

func (s *delayedChunkSource) Chunk(ctx context.Context, offset int64, limit int64) ([]byte, error) {
	if delay := s.delays[offset]; delay > 0 {
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		}
	}
	s.mu.Lock()
	s.seen = append(s.seen, offset)
	s.mu.Unlock()
	return s.testChunkSource.Chunk(ctx, offset, limit)
}

func (s *delayedChunkSource) offsets() []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]int64(nil), s.seen...)
	slices.Sort(out)
	return out
}

type blockingChunkSource struct {
	started  chan struct{}
	canceled chan struct{}
	once     sync.Once
}

func (s *blockingChunkSource) ChunkSize(start, end int64) int64 {
	return 1024
}

func (s *blockingChunkSource) Chunk(ctx context.Context, offset int64, limit int64) ([]byte, error) {
	s.once.Do(func() { close(s.started) })
	<-ctx.Done()
	close(s.canceled)
	return nil, ctx.Err()
}

type sleepingChunkSource struct {
	sleep time.Duration
}

func (s *sleepingChunkSource) ChunkSize(start, end int64) int64 {
	return 1024
}

func (s *sleepingChunkSource) Chunk(ctx context.Context, offset int64, limit int64) ([]byte, error) {
	timer := time.NewTimer(s.sleep)
	defer timer.Stop()
	select {
	case <-timer.C:
		return bytes.Repeat([]byte{0x31}, int(limit)), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
