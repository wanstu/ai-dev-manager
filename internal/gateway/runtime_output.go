package gateway

import (
	"bytes"
	"sync"
)

type ownerOutputBuffer struct {
	mu        sync.Mutex
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func newOwnerOutputBuffer(limit int) *ownerOutputBuffer {
	if limit <= 0 {
		limit = 120000
	}
	return &ownerOutputBuffer{limit: limit}
}

func (b *ownerOutputBuffer) Write(p []byte) (int, error) {
	if b == nil {
		return len(p), nil
	}
	original := len(p)
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		if len(p) > 0 {
			b.truncated = true
		}
		return original, nil
	}
	if len(p) > remaining {
		p = p[:remaining]
		b.truncated = true
	}
	_, _ = b.buf.Write(p)
	return original, nil
}

func (b *ownerOutputBuffer) Snapshot() (string, bool) {
	if b == nil {
		return "", false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String(), b.truncated
}
