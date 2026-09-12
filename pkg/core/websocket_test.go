package core

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
)

func TestMatchWebSocketRoutePattern(t *testing.T) {
	params, ok := matchRoutePattern("/rooms/{room}/users/{id}", "/rooms/general/users/42")
	if !ok || len(params) != 2 || params[0] != "general" || params[1] != "42" {
		t.Fatalf("matchRoutePattern returned %v, %v", params, ok)
	}
	if _, ok := matchRoutePattern("/rooms/{room}", "/rooms/a/messages"); ok {
		t.Fatal("route pattern accepted extra path segments")
	}
}

func TestWebSocketCloseInvokesCloser(t *testing.T) {
	r := NewRuntime()
	closed := false
	instance := &Instance{Fields: map[string]interface{}{"_closer": func() error {
		closed = true
		return nil
	}}}
	if got := r.executeWebSocketMethod(instance, "close", nil); got != true || !closed {
		t.Fatalf("close result=%v closed=%v", got, closed)
	}
}

func TestWebSocketOnMessageCapturedFunction(t *testing.T) {
	r := NewRuntime()
	instance := &Instance{Fields: make(map[string]interface{})}
	closure := &CapturedFunction{}
	if got := r.executeWebSocketMethod(instance, "onMessage", []interface{}{closure}); got != true {
		t.Fatalf("onMessage result=%v, expected true", got)
	}
	if instance.Fields["_on_message"] != closure {
		t.Fatalf("instance._on_message was not set to closure")
	}
}

func TestWebSocketLifecycleOrdersCallbacksAndClosesExactlyOnce(t *testing.T) {
	r := NewRuntime()
	defer r.Free()
	events := make([]string, 0, 2)
	closeCalls := 0
	reads := 0
	instance := &Instance{Fields: make(map[string]interface{})}
	instance.Fields["_on_message"] = NativeHandler(func(_ *Runtime, _ *Instance, _ string, args []interface{}) interface{} {
		events = append(events, "message:"+args[0].(string))
		return nil
	})
	instance.Fields["_on_close"] = NativeHandler(func(_ *Runtime, _ *Instance, _ string, _ []interface{}) interface{} {
		events = append(events, "close")
		return nil
	})
	lifecycle := &webSocketLifecycle{
		runtime:  r,
		instance: instance,
		reader: func() (int, []byte, error) {
			reads++
			if reads == 1 {
				return 1, []byte("hello"), nil
			}
			return 0, nil, errors.New("closed")
		},
		closer: func() error {
			closeCalls++
			return nil
		},
	}
	instance.Fields["_closer"] = lifecycle.close
	subscribeWebSocketChannel("room", instance)
	lifecycle.readMessages()
	if err := lifecycle.close(); err != nil {
		t.Fatal(err)
	}
	lifecycle.cleanup()
	if closeCalls != 1 {
		t.Fatalf("connection closed %d times, want once", closeCalls)
	}
	if len(events) != 2 || events[0] != "message:hello" || events[1] != "close" {
		t.Fatalf("callback order = %v", events)
	}
	if count := webSocketChannelSubscriberCount("room"); count != 0 {
		t.Fatalf("subscription leaked after cleanup: %d", count)
	}
}

func TestWebSocketCallbackPanicIsIsolatedAndCleanupStillRuns(t *testing.T) {
	r := NewRuntime()
	defer r.Free()
	closed := false
	closeCallback := false
	instance := &Instance{Fields: map[string]interface{}{
		"_on_message": NativeHandler(func(_ *Runtime, _ *Instance, _ string, _ []interface{}) interface{} {
			panic("callback failed")
		}),
		"_on_close": NativeHandler(func(_ *Runtime, _ *Instance, _ string, _ []interface{}) interface{} {
			closeCallback = true
			return nil
		}),
	}}
	lifecycle := &webSocketLifecycle{
		runtime:  r,
		instance: instance,
		reader: func() (int, []byte, error) {
			return 1, []byte("boom"), nil
		},
		closer: func() error {
			closed = true
			return nil
		},
	}
	lifecycle.readMessages()
	lifecycle.cleanup()
	if !closeCallback || !closed {
		t.Fatalf("cleanup after callback panic: onClose=%v closed=%v", closeCallback, closed)
	}
}

func TestWebSocketRealJossCallbacksAndRouteParams(t *testing.T) {
	r := NewRuntime()
	defer r.Free()

	source := `
public class ChatWsController {
    public func handle(WebSocket $ws, string $room): void {
        $ws->subscribe($room);
        $ws->onMessage(func(string $msg) {
            $ws->publish($room, "room=" . $room . ":" . $msg);
        });
        return;
    }
}
`
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if errors := p.Errors(); len(errors) > 0 {
		t.Fatalf("parse errors: %v", errors)
	}
	for _, stmt := range prog.Statements {
		if cls, ok := stmt.(*parser.ClassStatement); ok {
			r.Classes[cls.Name.Value] = cls
		}
	}

	published := make([]string, 0)
	var pubMu sync.Mutex

	// Mock receiver instance on the same channel
	receiver := &Instance{
		Class: r.Classes["WebSocket"],
		Fields: map[string]interface{}{
			"_sender": func(msg interface{}) error {
				pubMu.Lock()
				defer pubMu.Unlock()
				published = append(published, fmt.Sprint(msg))
				return nil
			},
		},
	}
	subscribeWebSocketChannel("gaming", receiver)
	defer unsubscribeWebSocketChannel("gaming", receiver)

	reads := 0
	reader := func() (int, []byte, error) {
		reads++
		if reads == 1 {
			return 1, []byte("play"), nil
		}
		return 0, nil, errors.New("closed")
	}

	sender := func(msg interface{}) error {
		return nil
	}

	closed := false
	closer := func() error {
		closed = true
		return nil
	}

	r.Routes["WS"] = map[string]interface{}{
		"/ws/{room}": map[string]interface{}{"handler": "ChatWsController@handle"},
	}
	r.DispatchWebSocket("/ws/gaming", nil, reader, sender, closer)

	if !closed {
		t.Fatal("expected connection to be closed")
	}

	pubMu.Lock()
	defer pubMu.Unlock()
	if len(published) != 1 || published[0] != "room=gaming:play" {
		t.Fatalf("published messages: %v, want ['room=gaming:play']", published)
	}
}

func TestWebSocketTwoConcurrentConnectionsIsolation(t *testing.T) {
	r := NewRuntime()
	defer r.Free()

	inst1 := &Instance{Class: r.Classes["WebSocket"], Fields: make(map[string]interface{})}
	inst2 := &Instance{Class: r.Classes["WebSocket"], Fields: make(map[string]interface{})}

	var rec1, rec2 []string
	var mu1, mu2 sync.Mutex

	inst1.Fields["_sender"] = func(msg interface{}) error {
		mu1.Lock()
		rec1 = append(rec1, fmt.Sprint(msg))
		mu1.Unlock()
		return nil
	}
	inst2.Fields["_sender"] = func(msg interface{}) error {
		mu2.Lock()
		rec2 = append(rec2, fmt.Sprint(msg))
		mu2.Unlock()
		return nil
	}

	subscribeWebSocketChannel("shared_chan", inst1)
	subscribeWebSocketChannel("shared_chan", inst2)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			publishWebSocketChannel("shared_chan", "msg_from_1", nil)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			publishWebSocketChannel("shared_chan", "msg_from_2", nil)
		}
	}()
	wg.Wait()

	// Unsubscribe inst1 completely
	unsubscribeWebSocketFromAllChannels(inst1)

	// inst2 must still be subscribed to shared_chan
	if count := webSocketChannelSubscriberCount("shared_chan"); count != 1 {
		t.Fatalf("expected 1 remaining subscriber, got %d", count)
	}

	// Another publish to shared_chan should be received by inst2 only
	mu2.Lock()
	beforeLen2 := len(rec2)
	mu2.Unlock()

	publishWebSocketChannel("shared_chan", "msg_final", nil)

	mu1.Lock()
	len1After := len(rec1)
	mu1.Unlock()
	mu2.Lock()
	len2After := len(rec2)
	mu2.Unlock()

	if len2After != beforeLen2+1 {
		t.Fatalf("inst2 did not receive final message: before=%d after=%d", beforeLen2, len2After)
	}

	// Clean up inst2
	unsubscribeWebSocketFromAllChannels(inst2)
	if count := webSocketChannelSubscriberCount("shared_chan"); count != 0 {
		t.Fatalf("expected 0 subscribers after inst2 cleanup, got %d", count)
	}
	_ = len1After
}
