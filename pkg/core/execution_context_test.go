package core

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jossecurity/joss/pkg/parser"
)

func TestExecutionContextCancelsEmptyInfiniteLoop(t *testing.T) {
	p := parser.NewParser(parser.NewLexer("while (true) {}"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parse: %v", p.Errors())
	}
	r := newRuntimeState().(*Runtime)
	defer r.Free()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	r.SetExecutionContext(ctx)
	done := make(chan interface{}, 1)
	go func() {
		defer func() { done <- recover() }()
		r.Execute(program)
	}()
	select {
	case recovered := <-done:
		if err, ok := recovered.(*JossError); !ok || err.Type != "ExecutionCancelled" {
			t.Fatalf("expected execution cancellation, got %#v", recovered)
		}
	case <-time.After(time.Second):
		t.Fatal("interpreter did not stop after context cancellation")
	}
}

func TestExecutionContextCancelsOutgoingHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		<-req.Context().Done()
	}))
	defer server.Close()
	probe, err := net.DialTimeout("tcp", server.Listener.Addr().String(), 250*time.Millisecond)
	if err != nil {
		t.Skipf("local HTTP sockets are unavailable in this environment: %v", err)
	}
	_ = probe.Close()
	r := newRuntimeState().(*Runtime)
	defer r.Free()
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	r.SetExecutionContext(ctx)
	defer func() {
		got, ok := recover().(*JossError)
		if !ok || got.Type != "ExecutionCancelled" {
			t.Fatalf("expected HTTP cancellation, got %#v", got)
		}
	}()
	r.performFullHttpRequest("GET", server.URL, "", nil, nil, 5, true)
}
