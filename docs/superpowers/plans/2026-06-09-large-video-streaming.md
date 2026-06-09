# Large Video Streaming Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add bounded Telegram chunk prefetching so large `/v/:fileid` videos seek and play more smoothly.

**Architecture:** Keep HTTP media proxy behavior unchanged and replace the Telegram layer's sequential range loop with a small internal prefetch reader. Runtime stream settings flow from `telegram.stream` config into `telegram.RuntimeConfig`, and `GotdClient.StreamVideoRange` uses those settings while preserving existing file-reference refresh behavior.

**Tech Stack:** Go standard library, gotd `tg.UploadGetFile`, existing config loader, existing `go test` suite.

---

## File Structure

- Modify `internal/config/config.go`: add `TelegramStreamConfig`, default values, and validation/default normalization through existing config flow.
- Modify `internal/config/config_test.go`: prove default and explicit `telegram.stream` config parsing.
- Modify `internal/telegram/options.go`: add stream runtime fields and normalize them for all gotd client users.
- Modify `internal/telegram/options_test.go`: prove stream config flows from config and normalizes invalid runtime values.
- Create `internal/telegram/range_reader.go`: implement chunk sizing, bounded concurrent prefetch, ordered reads, clipping, cancellation, and timeout errors.
- Create `internal/telegram/range_reader_test.go`: focused unit tests for chunk sizing, clipping, ordering, close cancellation, and chunk timeout.
- Modify `internal/telegram/media_proxy.go`: replace `streamFileRange` sequential chunk loop with the new reader and `io.Copy`.

---

### Task 1: Stream Runtime Config

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/telegram/options.go`
- Modify: `internal/telegram/options_test.go`

- [ ] **Step 1: Write failing config tests**

Add these assertions to `TestLoadAppliesTelegramRuntimeDefaults` in `internal/config/config_test.go` after the rate limit checks:

```go
	if cfg.Telegram.Stream.Concurrency != 2 {
		t.Fatalf("telegram stream concurrency = %d, want 2", cfg.Telegram.Stream.Concurrency)
	}
	if cfg.Telegram.Stream.Buffers != 4 {
		t.Fatalf("telegram stream buffers = %d, want 4", cfg.Telegram.Stream.Buffers)
	}
	if time.Duration(cfg.Telegram.Stream.ChunkTimeout) != 20*time.Second {
		t.Fatalf("telegram stream chunk timeout = %s, want 20s", cfg.Telegram.Stream.ChunkTimeout)
	}
```

Add this YAML and these assertions to `TestLoadAppliesTelegramRuntimeConfig`:

```yaml
  stream:
    concurrency: 3
    buffers: 6
    chunk_timeout: 15s
```

```go
	if cfg.Telegram.Stream.Concurrency != 3 {
		t.Fatalf("telegram stream concurrency = %d, want 3", cfg.Telegram.Stream.Concurrency)
	}
	if cfg.Telegram.Stream.Buffers != 6 {
		t.Fatalf("telegram stream buffers = %d, want 6", cfg.Telegram.Stream.Buffers)
	}
	if time.Duration(cfg.Telegram.Stream.ChunkTimeout) != 15*time.Second {
		t.Fatalf("telegram stream chunk timeout = %s, want 15s", cfg.Telegram.Stream.ChunkTimeout)
	}
```

- [ ] **Step 2: Run config tests to verify they fail**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/config
```

Expected: fail because `cfg.Telegram.Stream` is undefined.

- [ ] **Step 3: Implement config struct and defaults**

In `internal/config/config.go`, extend `TelegramConfig` and add `TelegramStreamConfig`:

```go
type TelegramConfig struct {
	Proxy            string                  `yaml:"proxy"`
	ReconnectTimeout Duration                `yaml:"reconnect_timeout"`
	DialTimeout      Duration                `yaml:"dial_timeout"`
	RateLimit        TelegramRateLimitConfig `yaml:"rate_limit"`
	Stream           TelegramStreamConfig    `yaml:"stream"`
}

type TelegramStreamConfig struct {
	Concurrency  int      `yaml:"concurrency"`
	Buffers      int      `yaml:"buffers"`
	ChunkTimeout Duration `yaml:"chunk_timeout"`
}
```

Update `defaultConfig` or `applyDefaults`, following the existing local pattern, so defaults become:

```go
cfg.Telegram.Stream.Concurrency = 2
cfg.Telegram.Stream.Buffers = 4
cfg.Telegram.Stream.ChunkTimeout = Duration(20 * time.Second)
```

Apply lower bounds in defaults/validation:

```go
if cfg.Telegram.Stream.Concurrency < 1 {
	cfg.Telegram.Stream.Concurrency = 1
}
if cfg.Telegram.Stream.Buffers < 1 {
	cfg.Telegram.Stream.Buffers = 1
}
if cfg.Telegram.Stream.ChunkTimeout.Std() <= 0 {
	cfg.Telegram.Stream.ChunkTimeout = Duration(20 * time.Second)
}
```

- [ ] **Step 4: Run config tests to verify they pass**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/config
```

Expected: pass.

- [ ] **Step 5: Write failing runtime mapping tests**

In `internal/telegram/options_test.go`, extend `TestRuntimeConfigFromConfig` with:

```go
		Stream: config.TelegramStreamConfig{
			Concurrency:  3,
			Buffers:      6,
			ChunkTimeout: config.Duration(15 * time.Second),
		},
```

Then assert:

```go
	if runtime.Stream.Concurrency != 3 {
		t.Fatalf("stream concurrency = %d, want 3", runtime.Stream.Concurrency)
	}
	if runtime.Stream.Buffers != 6 {
		t.Fatalf("stream buffers = %d, want 6", runtime.Stream.Buffers)
	}
	if runtime.Stream.ChunkTimeout != 15*time.Second {
		t.Fatalf("stream chunk timeout = %s, want 15s", runtime.Stream.ChunkTimeout)
	}
```

Add a new test:

```go
func TestNormalizeRuntimeConfigAppliesStreamDefaults(t *testing.T) {
	runtime := normalizeRuntimeConfig(RuntimeConfig{
		Stream: StreamConfig{
			Concurrency:  -1,
			Buffers:      0,
			ChunkTimeout: -time.Second,
		},
	})

	if runtime.Stream.Concurrency != 1 {
		t.Fatalf("stream concurrency = %d, want 1", runtime.Stream.Concurrency)
	}
	if runtime.Stream.Buffers != 1 {
		t.Fatalf("stream buffers = %d, want 1", runtime.Stream.Buffers)
	}
	if runtime.Stream.ChunkTimeout != 20*time.Second {
		t.Fatalf("stream chunk timeout = %s, want 20s", runtime.Stream.ChunkTimeout)
	}
}
```

- [ ] **Step 6: Run runtime mapping tests to verify they fail**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/telegram -run 'TestRuntimeConfigFromConfig|TestNormalizeRuntimeConfigAppliesStreamDefaults' -count=1
```

Expected: fail because `StreamConfig` and runtime stream fields are undefined.

- [ ] **Step 7: Implement runtime stream mapping**

In `internal/telegram/options.go`, add:

```go
type StreamConfig struct {
	Concurrency  int
	Buffers      int
	ChunkTimeout time.Duration
}

type RuntimeConfig struct {
	Proxy            string
	ReconnectTimeout time.Duration
	DialTimeout      time.Duration
	RateLimit        RateLimitConfig
	Stream           StreamConfig
}
```

Update `DefaultRuntimeConfig`:

```go
		Stream: StreamConfig{
			Concurrency:  2,
			Buffers:      4,
			ChunkTimeout: 20 * time.Second,
		},
```

Update `RuntimeConfigFromConfig`:

```go
		Stream: StreamConfig{
			Concurrency:  cfg.Stream.Concurrency,
			Buffers:      cfg.Stream.Buffers,
			ChunkTimeout: cfg.Stream.ChunkTimeout.Std(),
		},
```

Update `normalizeRuntimeConfig`:

```go
	if cfg.Stream.Concurrency < 1 {
		cfg.Stream.Concurrency = 1
	}
	if cfg.Stream.Buffers < 1 {
		cfg.Stream.Buffers = 1
	}
	if cfg.Stream.ChunkTimeout <= 0 {
		cfg.Stream.ChunkTimeout = defaults.Stream.ChunkTimeout
	}
```

- [ ] **Step 8: Run runtime mapping tests to verify they pass**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/config ./internal/telegram -run 'TestLoadAppliesTelegramRuntime|TestRuntimeConfigFromConfig|TestNormalizeRuntimeConfigAppliesStreamDefaults' -count=1
```

Expected: pass.

- [ ] **Step 9: Commit Task 1**

```bash
git add internal/config/config.go internal/config/config_test.go internal/telegram/options.go internal/telegram/options_test.go
git commit -m "feat: add telegram stream runtime config"
```

---

### Task 2: Telegram Range Prefetch Reader

**Files:**
- Create: `internal/telegram/range_reader.go`
- Create: `internal/telegram/range_reader_test.go`

- [ ] **Step 1: Write failing chunk sizing and clipping tests**

Create `internal/telegram/range_reader_test.go` with:

```go
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
```

Add the test helper in the same file:

```go
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
```

- [ ] **Step 2: Run reader tests to verify they fail**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/telegram -run 'TestStreamChunkSize|TestRangePrefetchReaderClipsUnalignedRange' -count=1
```

Expected: fail because `streamChunkSize` and `newRangePrefetchReader` are undefined.

- [ ] **Step 3: Implement minimal range reader**

Create `internal/telegram/range_reader.go`:

```go
package telegram

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

var (
	ErrStreamAbandoned = errors.New("stream abandoned")
	ErrChunkTimeout    = errors.New("chunk fetch timed out")
)

type streamChunkSource interface {
	Chunk(ctx context.Context, offset int64, limit int64) ([]byte, error)
	ChunkSize(start, end int64) int64
}

type streamBuffer struct {
	data []byte
	pos  int
}

type rangePrefetchReader struct {
	ctx         context.Context
	cancel      context.CancelFunc
	source      streamChunkSource
	chunkSize   int64
	nextOffset  int64
	totalParts  int
	currentPart int
	limit       int64
	leftCut     int64
	rightCut    int64
	concurrency int
	timeout     time.Duration
	buffers     chan *streamBuffer
	current     *streamBuffer
	closeOnce   sync.Once
	errMu       sync.Mutex
	err         error
}

func streamChunkSize(start, end int64) int64 {
	chunkSize := int64(1024 * 1024)
	span := end - start
	for chunkSize > 1024 && chunkSize > span {
		chunkSize /= 2
	}
	return chunkSize
}

func newRangePrefetchReader(ctx context.Context, start int64, end int64, cfg StreamConfig, source streamChunkSource) *rangePrefetchReader {
	cfg = normalizeStreamConfig(cfg)
	chunkSize := source.ChunkSize(start, end)
	alignedOffset := start - (start % chunkSize)
	readerCtx, cancel := context.WithCancel(ctx)
	r := &rangePrefetchReader{
		ctx:         readerCtx,
		cancel:      cancel,
		source:      source,
		chunkSize:   chunkSize,
		nextOffset:  alignedOffset,
		totalParts:  int((end - alignedOffset + chunkSize) / chunkSize),
		limit:       end - start + 1,
		leftCut:     start - alignedOffset,
		rightCut:    (end % chunkSize) + 1,
		concurrency: cfg.Concurrency,
		timeout:     cfg.ChunkTimeout,
		buffers:     make(chan *streamBuffer, cfg.Buffers),
	}
	go r.fill()
	return r
}

func normalizeStreamConfig(cfg StreamConfig) StreamConfig {
	defaults := DefaultRuntimeConfig().Stream
	if cfg.Concurrency < 1 {
		cfg.Concurrency = 1
	}
	if cfg.Buffers < 1 {
		cfg.Buffers = 1
	}
	if cfg.ChunkTimeout <= 0 {
		cfg.ChunkTimeout = defaults.ChunkTimeout
	}
	return cfg
}

func (r *rangePrefetchReader) Read(p []byte) (int, error) {
	if r.limit <= 0 {
		return 0, io.EOF
	}
	for r.current == nil || r.current.pos >= len(r.current.data) {
		select {
		case buf, ok := <-r.buffers:
			if !ok {
				if err := r.loadErr(); err != nil {
					return 0, err
				}
				return 0, ErrStreamAbandoned
			}
			r.current = buf
		case <-r.ctx.Done():
			if err := r.loadErr(); err != nil {
				return 0, err
			}
			return 0, r.ctx.Err()
		}
	}
	n := copy(p, r.current.data[r.current.pos:])
	r.current.pos += n
	r.limit -= int64(n)
	if r.limit <= 0 {
		return n, io.EOF
	}
	return n, nil
}

func (r *rangePrefetchReader) Close() error {
	r.closeOnce.Do(r.cancel)
	return nil
}

func (r *rangePrefetchReader) fill() {
	defer close(r.buffers)
	for r.currentPart < r.totalParts {
		if err := r.fillBatch(); err != nil {
			r.setErr(err)
			r.cancel()
			return
		}
	}
}

func (r *rangePrefetchReader) setErr(err error) {
	r.errMu.Lock()
	defer r.errMu.Unlock()
	r.err = err
}

func (r *rangePrefetchReader) loadErr() error {
	r.errMu.Lock()
	defer r.errMu.Unlock()
	return r.err
}

func (r *rangePrefetchReader) fillBatch() error {
	batchSize := r.concurrency
	if remaining := r.totalParts - r.currentPart; remaining < batchSize {
		batchSize = remaining
	}
	results := make([]*streamBuffer, batchSize)
	errs := make(chan error, batchSize)
	var wg sync.WaitGroup
	for i := 0; i < batchSize; i++ {
		i := i
		partNo := r.currentPart + i
		offset := r.nextOffset + int64(i)*r.chunkSize
		wg.Add(1)
		go func() {
			defer wg.Done()
			chunkCtx, cancel := context.WithTimeout(r.ctx, r.timeout)
			defer cancel()
			chunk, err := r.source.Chunk(chunkCtx, offset, r.chunkSize)
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					errs <- fmt.Errorf("chunk %d: %w", partNo, ErrChunkTimeout)
					return
				}
				errs <- err
				return
			}
			if len(chunk) == 0 {
				errs <- io.ErrUnexpectedEOF
				return
			}
			chunk = r.clipChunk(partNo, chunk)
			results[i] = &streamBuffer{data: chunk}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			return err
		}
	}
	for _, result := range results {
		select {
		case r.buffers <- result:
		case <-r.ctx.Done():
			return r.ctx.Err()
		}
	}
	r.currentPart += batchSize
	r.nextOffset += int64(batchSize) * r.chunkSize
	return nil
}

func (r *rangePrefetchReader) clipChunk(partNo int, chunk []byte) []byte {
	first := partNo == 0
	last := partNo == r.totalParts-1
	if first && last {
		return chunk[r.leftCut:r.rightCut]
	}
	if first {
		return chunk[r.leftCut:]
	}
	if last {
		return chunk[:r.rightCut]
	}
	return chunk
}
```

- [ ] **Step 4: Run reader tests to verify they pass**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/telegram -run 'TestStreamChunkSize|TestRangePrefetchReaderClipsUnalignedRange' -count=1
```

Expected: pass.

- [ ] **Step 5: Write failing ordering, close, and timeout tests**

Append these tests to `internal/telegram/range_reader_test.go`:

```go
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
```

Append these helpers:

```go
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
```

- [ ] **Step 6: Run new reader tests to verify they fail if behavior is incomplete**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/telegram -run 'TestRangePrefetchReaderPreservesOrder|TestRangePrefetchReaderCloseCancelsChunkFetch|TestRangePrefetchReaderReportsChunkTimeout' -count=1
```

Expected: pass, proving the reader preserves order, cancels in-flight chunk fetches on close, and wraps chunk deadline failures with `ErrChunkTimeout`.

- [ ] **Step 7: Run all telegram tests**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/telegram -count=1
```

Expected: pass.

- [ ] **Step 8: Commit Task 2**

```bash
git add internal/telegram/range_reader.go internal/telegram/range_reader_test.go
git commit -m "feat: add telegram range prefetch reader"
```

---

### Task 3: Use Prefetch Reader For Video Streaming

**Files:**
- Modify: `internal/telegram/media_proxy.go`
- Test: `internal/telegram/range_reader_test.go`

- [ ] **Step 1: Write failing stream integration test**

Add this test to `internal/telegram/range_reader_test.go`:

```go
func TestStreamRangeFromSourceUsesPrefetchReader(t *testing.T) {
	src := newTestChunkSource(4, 1024)
	var out bytes.Buffer

	written, err := streamRangeFromSource(context.Background(), &out, 100, 2201, StreamConfig{
		Concurrency:  2,
		Buffers:      2,
		ChunkTimeout: time.Second,
	}, src)
	if err != nil {
		t.Fatalf("streamRangeFromSource returned error: %v", err)
	}
	if written != 2201 {
		t.Fatalf("written = %d, want 2201", written)
	}
	if !bytes.Equal(out.Bytes(), src.bytes[100:2301]) {
		t.Fatal("streamed bytes did not match requested range")
	}
}
```

- [ ] **Step 2: Run integration test to verify it fails**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/telegram -run TestStreamRangeFromSourceUsesPrefetchReader -count=1
```

Expected: fail because `streamRangeFromSource` is undefined.

- [ ] **Step 3: Wire prefetch reader into `streamFileRange`**

In `internal/telegram/media_proxy.go`, replace the sequential body of `streamFileRange` with a reader-based implementation. Keep the public `StreamVideoRange` interface unchanged.

Use this shape:

```go
type telegramFileChunkSource struct {
	api *tg.Client
	loc tg.InputFileLocationClass
}

func (s telegramFileChunkSource) ChunkSize(start, end int64) int64 {
	return streamChunkSize(start, end)
}

func (s telegramFileChunkSource) Chunk(ctx context.Context, offset int64, limit int64) ([]byte, error) {
	res, err := s.api.UploadGetFile(ctx, &tg.UploadGetFileRequest{
		Location: s.loc,
		Offset:   offset,
		Limit:    int(limit),
		Precise:  true,
	})
	if err != nil {
		return nil, err
	}
	f, ok := res.(*tg.UploadFile)
	if !ok {
		return nil, fmt.Errorf("unexpected upload.getFile result %T", res)
	}
	if len(f.Bytes) == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	return f.Bytes, nil
}

func streamRangeFromSource(ctx context.Context, w io.Writer, offset int64, remain int64, cfg StreamConfig, source streamChunkSource) (int64, error) {
	if remain <= 0 {
		return 0, nil
	}
	reader := newRangePrefetchReader(ctx, offset, offset+remain-1, cfg, source)
	defer reader.Close()
	return io.CopyN(w, reader, remain)
}

func streamFileRange(ctx context.Context, api *tg.Client, w io.Writer, loc tg.InputFileLocationClass, offset int64, remain int64, cfg StreamConfig) (int64, error) {
	return streamRangeFromSource(ctx, w, offset, remain, cfg, telegramFileChunkSource{api: api, loc: loc})
}
```

Update both call sites in `StreamVideoRange`:

```go
written, err := streamFileRange(ctx, client.API(), w, loc, offset, length, g.runtime.Stream)
```

```go
_, err = streamFileRange(ctx, client.API(), w, documentFileLocation(videoFileFromDocument(doc), ""), offset, length, g.runtime.Stream)
```

- [ ] **Step 4: Run telegram tests**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/telegram -count=1
```

Expected: pass.

- [ ] **Step 5: Run API media proxy tests**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/api -run 'TestServeTelegramVideo|TestParseRange|TestTelegramMedia' -count=1
```

Expected: pass.

- [ ] **Step 6: Commit Task 3**

```bash
git add internal/telegram/media_proxy.go internal/telegram/range_reader_test.go
git commit -m "feat: prefetch telegram video ranges"
```

---

### Task 4: Full Verification

**Files:**
- All changed files from Tasks 1-3

- [ ] **Step 1: Run full Go test suite**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./...
```

Expected: pass.

- [ ] **Step 2: Check git status**

Run:

```bash
git status --short
```

Expected: no output.

- [ ] **Step 3: Summarize commits**

Run:

```bash
git log --oneline --decorate -5
```

Expected: the branch contains the design commit, stream config commit, range reader commit, and video range prefetch commit.
