package core

import (
	"context"
	"fmt"
	"runtime"

	"github.com/jossecurity/joss/pkg/diagnostics"
)

// NewChannel creates a new Channel with initialized closing signal.
func NewChannel(size int) *Channel {
	return &Channel{
		Ch:      make(chan interface{}, size),
		closing: make(chan struct{}),
	}
}

func (c *Channel) ensureClosing() chan struct{} {
	c.initMu.Do(func() {
		if c.closing == nil {
			c.closing = make(chan struct{})
		}
	})
	return c.closing
}

// IsClosed returns true if the channel has been marked closed.
func (c *Channel) IsClosed() bool {
	return c == nil || c.closed.Load()
}

// TrySend contains Go's send-on-closed panic at the native boundary. A send
// can still block when no receiver is available, as documented by Joss.
func (c *Channel) TrySend(value interface{}) (err error) {
	return c.TrySendContext(context.Background(), value)
}

func (c *Channel) TrySendContext(ctx context.Context, value interface{}) (err error) {
	closing, err := c.beginSend()
	if err != nil {
		return err
	}
	defer c.endSend()

	defer func() {
		if recovered := recover(); recovered != nil {
			err = &JossError{Type: "ChannelError", Code: diagnostics.CodeChannelClosed, Message: fmt.Sprintf("No se puede enviar al canal: %v", recovered)}
		}
	}()

	select {
	case c.Ch <- value:
		return nil
	case <-closing:
		return closedChannelError()
	case <-ctx.Done():
		return &JossError{Type: "ExecutionCancelled", Message: ctx.Err().Error()}
	}
}

func closedChannelError() *JossError {
	return &JossError{Type: "ChannelError", Code: diagnostics.CodeChannelClosed, Message: "No se puede enviar al canal: el canal está cerrado"}
}

// beginSend reserves a sender until the caller finishes its send or select.
// TryClose waits for every reservation before closing the underlying Go channel.
func (c *Channel) beginSend() (chan struct{}, error) {
	if c == nil || c.Ch == nil {
		return nil, &JossError{Type: "ChannelError", Code: diagnostics.CodeChannelClosed, Message: "El canal no está disponible"}
	}
	if c.closed.Load() {
		return nil, closedChannelError()
	}
	closing := c.ensureClosing()
	c.senders.Add(1)
	if c.closed.Load() {
		c.endSend()
		return nil, closedChannelError()
	}
	return closing, nil
}

func (c *Channel) endSend() {
	c.senders.Add(-1)
}

func (c *Channel) Send(value interface{}) {
	if err := c.TrySend(value); err != nil {
		panic(err)
	}
}

// TryClose makes close idempotency failures observable without leaking a Go
// panic. It is also used by host cleanup paths, which may ignore the error.
func (c *Channel) TryClose() (err error) {
	if c == nil || c.Ch == nil {
		return &JossError{Type: "ChannelError", Code: diagnostics.CodeChannelClosed, Message: "El canal no está disponible"}
	}
	if !c.closed.CompareAndSwap(false, true) {
		return &JossError{Type: "ChannelError", Code: diagnostics.CodeChannelClosed, Message: "El canal ya está cerrado"}
	}
	closing := c.ensureClosing()
	close(closing)

	// Wait for active senders to observe closing and exit select before closing c.Ch
	for c.senders.Load() > 0 {
		runtime.Gosched()
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			err = &JossError{Type: "ChannelError", Code: diagnostics.CodeChannelClosed, Message: fmt.Sprintf("El canal ya está cerrado: %v", recovered)}
		}
	}()
	close(c.Ch)
	return nil
}

func (c *Channel) Close() {
	if err := c.TryClose(); err != nil {
		panic(err)
	}
}
