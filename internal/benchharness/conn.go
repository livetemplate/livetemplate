// Package benchharness provides test-support types for honest benchmarks:
// doubles that run the real pipeline and fake only the syscall boundary.
//
// The package deliberately imports nothing outside the standard library so
// that in-package tests of both the root livetemplate package and
// internal/session can use it without import cycles.
package benchharness

import (
	"errors"
	"sync"
	"sync/atomic"
)

// ErrClosed is returned by Conn methods after Close.
var ErrClosed = errors.New("benchharness: conn closed")

// Conn is a WebSocket connection double that is real everywhere except the
// syscall: reads are scripted by the harness via FeedRead, and writes arrive
// fully serialized through the real writePump, are counted, and are discarded
// exactly where a production connection would hand the bytes to the kernel.
//
// It structurally satisfies both livetemplate.WSConn and session.WSConn
// (identical three-method sets, compile-time-synced in ws.go) while importing
// neither package.
type Conn struct {
	reads     chan []byte
	closed    chan struct{}
	closeOnce sync.Once

	writes       chan struct{}
	bytesWritten atomic.Int64
	msgsWritten  atomic.Int64
}

// NewConn returns a Conn ready to be handed to the session registry or a
// fake WSUpgrader.
func NewConn() *Conn {
	return &Conn{
		reads:  make(chan []byte, 16),
		closed: make(chan struct{}),
		// Writes is a lockstep wakeup signal ("at least one write landed
		// since you last awaited"), not a counter — drivers that need exact
		// totals read MsgsWritten/BytesWritten instead.
		writes: make(chan struct{}, 1),
	}
}

// FeedRead schedules one incoming frame; the next ReadMessage returns it.
func (c *Conn) FeedRead(frame []byte) error {
	select {
	case c.reads <- frame:
		return nil
	case <-c.closed:
		return ErrClosed
	}
}

// ReadMessage blocks until a frame is fed via FeedRead or the conn closes.
// The close error is a plain error (not a WSCloseError), which the event
// loop treats as a silent client disconnect.
func (c *Conn) ReadMessage() (int, []byte, error) {
	select {
	case frame := <-c.reads:
		return 1, frame, nil // 1 = RFC 6455 text message
	case <-c.closed:
		return 0, nil, ErrClosed
	}
}

// WriteMessage receives the fully serialized frame from the real write pump
// and discards it at the syscall boundary, recording counters and signaling
// Writes so drivers can await a response reaching the wire.
func (c *Conn) WriteMessage(_ int, data []byte) error {
	select {
	case <-c.closed:
		return ErrClosed
	default:
	}
	c.bytesWritten.Add(int64(len(data)))
	c.msgsWritten.Add(1)
	select {
	case c.writes <- struct{}{}:
	default: // no driver awaiting; counters still advance
	}
	return nil
}

// Writes delivers one tick per WriteMessage, letting a driver block until
// the pipeline's response has actually reached the write boundary.
func (c *Conn) Writes() <-chan struct{} { return c.writes }

// BytesWritten reports the total serialized bytes that reached WriteMessage.
func (c *Conn) BytesWritten() int64 { return c.bytesWritten.Load() }

// MsgsWritten reports the number of frames that reached WriteMessage.
func (c *Conn) MsgsWritten() int64 { return c.msgsWritten.Load() }

// Close unblocks pending reads and fails subsequent operations. Idempotent.
func (c *Conn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return nil
}
