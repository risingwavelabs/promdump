package prompush

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"
)

type PushWorker struct {
	vmEndpoint string
	c          chan []byte
	buf        bytes.Buffer
	cnt        int
	mu         sync.Mutex
	noop       bool
}

func NewPushWorker(ctx context.Context, vmEndpoint string, batchSize int, noop bool) *PushWorker {
	w := &PushWorker{
		vmEndpoint: vmEndpoint,
		c:          make(chan []byte, batchSize),
		cnt:        0,
		noop:       noop,
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case line := <-w.c:
				if w.cnt == batchSize {
					if err := w.Flush(ctx); err != nil {
						log.Printf("failed to flush: %s", err)
					}
				}
				w.Append(line)
			}
		}
	}()

	return w
}

func (w *PushWorker) Append(line []byte) {
	w.buf.Write(line)
	w.cnt++
}

func (w *PushWorker) Push(line []byte) {
	w.c <- line
}

func (w *PushWorker) Flush(ctx context.Context) error {
	if w.noop {
		return w.FlushNoops(ctx)
	}
	return w.FlushToEndpoint(ctx)
}

func (w *PushWorker) FlushNoops(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf.Len() == 0 {
		return nil
	}

	// reset the buffer
	w.buf.Reset()
	w.cnt = 0
	return nil
}

func (w *PushWorker) FlushToEndpoint(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf.Len() == 0 {
		return nil
	}

	// Create a copy of the buffer data to avoid modification during HTTP transfer
	data := make([]byte, w.buf.Len())
	copy(data, w.buf.Bytes())

	// Use http.Client directly instead of http.Post to keep mutex locked during transmission
	req, err := http.NewRequestWithContext(ctx, "POST", w.vmEndpoint+"/api/v1/import", bytes.NewReader(data))
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}
	req.Header.Set("Content-Type", "application/jsonl")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to push metrics")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to push metrics: status=%d body=%s", resp.StatusCode, string(body))
	}

	// reset the buffer
	w.buf.Reset()
	w.cnt = 0
	return nil
}

func (w *PushWorker) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = w.Flush(ctx)
	close(w.c)
}
