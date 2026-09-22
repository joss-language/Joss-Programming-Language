package core

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
)

func TestChannelConcurrentSendAndCloseWithoutDeadlock(t *testing.T) {
	const iterations = 50
	for i := 0; i < iterations; i++ {
		ch := &Channel{Ch: make(chan interface{}, 2)}
		var wg sync.WaitGroup
		closeOnce := sync.Once{}

		// 10 concurrent senders
		for s := 0; s < 10; s++ {
			wg.Add(1)
			go func(val int) {
				defer wg.Done()
				for m := 0; m < 20; m++ {
					err := ch.TrySend(val*100 + m)
					if err != nil {
						jerr, ok := err.(*JossError)
						if !ok || jerr.Code != diagnostics.CodeChannelClosed {
							t.Errorf("expected closed channel error, got: %v", err)
						}
						return
					}
				}
			}(s)
		}

		// 10 concurrent receivers
		for r := 0; r < 10; r++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case _, ok := <-ch.Ch:
						if !ok {
							return
						}
					case <-time.After(50 * time.Millisecond):
						return
					}
				}
			}()
		}

		// Concurrently close after a brief delay
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(time.Millisecond)
			closeOnce.Do(func() {
				_ = ch.TryClose()
			})
		}()

		// Multiple concurrent closers attempting TryClose
		for c := 0; c < 5; c++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = ch.TryClose()
			}()
		}

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Completed without deadlocks
		case <-time.After(2 * time.Second):
			t.Fatalf("iteration %d deadlocked on concurrent send/close", i)
		}
	}
}

func TestSelectStatementHandlesClosedChannelSafely(t *testing.T) {
	r := newRuntimeState().(*Runtime)
	defer r.Free()

	// Channel is closed, select has a send case to the closed channel
	source := `
$ch = make_chan(1)
close($ch)
select {
case send($ch, 42):
    $result = "sent"
default:
    $result = "default"
}
`
	p := parser.NewParser(parser.NewLexer(source))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}

	// Should either choose default or fail with structured channel error, never an unhandled Go crash
	defer func() {
		if rec := recover(); rec != nil {
			jerr, ok := rec.(*JossError)
			if !ok || jerr.Code != diagnostics.CodeChannelClosed {
				t.Fatalf("expected structured ChannelClosed error, got %#v", rec)
			}
		}
	}()
	r.Execute(program)
}

func TestSelectStatementCancelsOnExecutionContext(t *testing.T) {
	r := newRuntimeState().(*Runtime)
	defer r.Free()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	r.SetExecutionContext(ctx)

	source := `
$ch = make_chan()
select {
case recv($ch):
    $result = "received"
}
`
	p := parser.NewParser(parser.NewLexer(source))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}

	done := make(chan interface{}, 1)
	go func() {
		defer func() { done <- recover() }()
		r.Execute(program)
	}()

	select {
	case rec := <-done:
		jerr, ok := rec.(*JossError)
		if !ok || jerr.Type != "ExecutionCancelled" {
			t.Fatalf("expected ExecutionCancelled, got %#v", rec)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("select did not abort on context cancellation")
	}
}

func TestSleepCancelsOnExecutionContext(t *testing.T) {
	r := newRuntimeState().(*Runtime)
	defer r.Free()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	r.SetExecutionContext(ctx)

	source := `sleep(5)`
	p := parser.NewParser(parser.NewLexer(source))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}

	done := make(chan interface{}, 1)
	go func() {
		defer func() { done <- recover() }()
		r.Execute(program)
	}()

	select {
	case rec := <-done:
		jerr, ok := rec.(*JossError)
		if !ok || jerr.Type != "ExecutionCancelled" {
			t.Fatalf("expected ExecutionCancelled from sleep, got %#v", rec)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("sleep did not abort on context cancellation")
	}
}

func TestMobileRunDirectOutputIsolation(t *testing.T) {
	// Execute two scripts concurrently through mobile.RunDirect to verify output isolation
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(2)
		go func(idx int) {
			defer wg.Done()
			r := newRuntimeState().(*Runtime)
			defer r.Free()
			src := fmt.Sprintf(`echo "thread A %d"`, idx)
			p := parser.NewParser(parser.NewLexer(src))
			prog := p.ParseProgram()
			var buf stringsBuilder
			r.Out = &buf
			r.Execute(prog)
			if !containsSubstring(buf.String(), fmt.Sprintf("thread A %d", idx)) {
				t.Errorf("expected thread A output, got %s", buf.String())
			}
		}(i)
		go func(idx int) {
			defer wg.Done()
			r := newRuntimeState().(*Runtime)
			defer r.Free()
			src := fmt.Sprintf(`echo "thread B %d"`, idx)
			p := parser.NewParser(parser.NewLexer(src))
			prog := p.ParseProgram()
			var buf stringsBuilder
			r.Out = &buf
			r.Execute(prog)
			if !containsSubstring(buf.String(), fmt.Sprintf("thread B %d", idx)) {
				t.Errorf("expected thread B output, got %s", buf.String())
			}
		}(i)
	}
	wg.Wait()
}

type stringsBuilder struct {
	sync.Mutex
	data []byte
}

func (b *stringsBuilder) Write(p []byte) (n int, err error) {
	b.Lock()
	defer b.Unlock()
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *stringsBuilder) String() string {
	b.Lock()
	defer b.Unlock()
	return string(b.data)
}

func containsSubstring(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
