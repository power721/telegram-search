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
