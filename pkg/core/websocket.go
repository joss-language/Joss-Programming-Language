package core

import (
	"fmt"
	"strings"
	"sync"

	"github.com/jossecurity/joss/pkg/parser"
)

type webSocketLifecycle struct {
	runtime  *Runtime
	instance *Instance
	reader   func() (int, []byte, error)
	closer   func() error
	closeMu  sync.Once
	closeErr error
}

func (l *webSocketLifecycle) close() error {
	l.closeMu.Do(func() {
		if l.closer != nil {
			l.closeErr = l.closer()
		}
	})
	return l.closeErr
}

func (l *webSocketLifecycle) cleanup() {
	if callback, ok := l.instance.Fields["_on_close"]; ok {
		l.runtime.callWebSocketCallback("onClose", callback, nil)
	}
	unsubscribeWebSocketFromAllChannels(l.instance)
	_ = l.close()
}

func (l *webSocketLifecycle) readMessages() {
	for {
		_, message, err := l.reader()
		if err != nil {
			fmt.Printf("[WS] Connection Error/Closed: %v\n", err)
			return
		}
		if callback, ok := l.instance.Fields["_on_message"]; ok {
			if !l.runtime.callWebSocketCallback("onMessage", callback, []interface{}{string(message)}) {
				return
			}
		}
	}
}

var webSocketChannels = struct {
	sync.RWMutex
	members map[string]map[*Instance]struct{}
}{
	members: make(map[string]map[*Instance]struct{}),
}

// WebSocket Implementation
func (r *Runtime) executeWebSocketMethod(instance *Instance, method string, args []interface{}) interface{} {
	switch method {
	case "broadcast":
		if len(args) > 0 {
			msg := args[0]
			if BroadcastFunc != nil {
				BroadcastFunc(msg)
				return true
			} else {
				fmt.Println("[WebSocket] Error: BroadcastFunc not initialized")
			}
		}
		return false

	case "send":
		// $ws->send(msg)
		if len(args) > 0 {
			msg := args[0]
			if connVal, ok := instance.Fields["_sender"]; ok {
				if sender, ok := connVal.(func(interface{}) error); ok {
					if err := sender(msg); err != nil {
						fmt.Printf("[WebSocket] Send error: %v\n", err)
						return false
					}
					return true
				}
			}
		}
		return false

	case "subscribe":
		if len(args) < 1 || instance == nil {
			return false
		}
		channel, ok := args[0].(string)
		channel = strings.TrimSpace(channel)
		if !ok || channel == "" {
			return false
		}
		subscribeWebSocketChannel(channel, instance)
		return true

	case "unsubscribe":
		if len(args) < 1 || instance == nil {
			return false
		}
		channel, ok := args[0].(string)
		channel = strings.TrimSpace(channel)
		if !ok || channel == "" {
			return false
		}
		unsubscribeWebSocketChannel(channel, instance)
		return true

	case "publish":
		if len(args) < 2 {
			return int64(0)
		}
		channel, ok := args[0].(string)
		channel = strings.TrimSpace(channel)
		if !ok || channel == "" {
			return int64(0)
		}
		var exclude *Instance
		if instance != nil {
			if _, connected := instance.Fields["_sender"]; connected {
				exclude = instance
			}
		}
		return int64(publishWebSocketChannel(channel, args[1], exclude))

	case "subscriberCount":
		if len(args) < 1 {
			return int64(0)
		}
		channel, ok := args[0].(string)
		channel = strings.TrimSpace(channel)
		if !ok || channel == "" {
			return int64(0)
		}
		return int64(webSocketChannelSubscriberCount(channel))

	case "onMessage":
		// $ws->onMessage(func($msg) { ... })
		if len(args) > 0 {
			// BoundMethod (e.g. $this.handle)
			if fn, ok := args[0].(*BoundMethod); ok {
				instance.Fields["_on_message"] = fn
				return true
			}
			// FunctionLiteral (Anonymous function)
			if fn, ok := args[0].(*parser.FunctionLiteral); ok {
				instance.Fields["_on_message"] = r.captureFunction(fn)
				return true
			}
			// CapturedFunction (Evaluated closure)
			if fn, ok := args[0].(*CapturedFunction); ok {
				instance.Fields["_on_message"] = fn
				return true
			}
			instance.Fields["_on_message"] = args[0]
			return true
		}
		return false

	case "onClose":
		// $ws->onClose(func() { ... })
		if len(args) > 0 {
			if fn, ok := args[0].(*BoundMethod); ok {
				instance.Fields["_on_close"] = fn
				return true
			}
			if fn, ok := args[0].(*parser.FunctionLiteral); ok {
				instance.Fields["_on_close"] = r.captureFunction(fn)
				return true
			}
			if fn, ok := args[0].(*CapturedFunction); ok {
				instance.Fields["_on_close"] = fn
				return true
			}
			instance.Fields["_on_close"] = args[0]
			return true
		}
		return false

	case "close":
		if closer, ok := instance.Fields["_closer"].(func() error); ok {
			if err := closer(); err != nil {
				fmt.Printf("[WebSocket] Error closing connection: %v\n", err)
				return false
			}
			return true
		}
		return false
	}
	return nil
}

func subscribeWebSocketChannel(channel string, instance *Instance) {
	webSocketChannels.Lock()
	defer webSocketChannels.Unlock()

	members := webSocketChannels.members[channel]
	if members == nil {
		members = make(map[*Instance]struct{})
		webSocketChannels.members[channel] = members
	}
	members[instance] = struct{}{}

	subscriptions, _ := instance.Fields["_subscriptions"].(map[string]struct{})
	if subscriptions == nil {
		subscriptions = make(map[string]struct{})
		instance.Fields["_subscriptions"] = subscriptions
	}
	subscriptions[channel] = struct{}{}
}

func unsubscribeWebSocketChannel(channel string, instance *Instance) {
	webSocketChannels.Lock()
	defer webSocketChannels.Unlock()
	unsubscribeWebSocketChannelLocked(channel, instance)
}

func unsubscribeWebSocketChannelLocked(channel string, instance *Instance) {
	if members := webSocketChannels.members[channel]; members != nil {
		delete(members, instance)
		if len(members) == 0 {
			delete(webSocketChannels.members, channel)
		}
	}
	if subscriptions, ok := instance.Fields["_subscriptions"].(map[string]struct{}); ok {
		delete(subscriptions, channel)
	}
}

func unsubscribeWebSocketFromAllChannels(instance *Instance) {
	webSocketChannels.Lock()
	defer webSocketChannels.Unlock()

	subscriptions, _ := instance.Fields["_subscriptions"].(map[string]struct{})
	for channel := range subscriptions {
		unsubscribeWebSocketChannelLocked(channel, instance)
	}
	delete(instance.Fields, "_subscriptions")
}

func publishWebSocketChannel(channel string, message interface{}, exclude *Instance) int {
	webSocketChannels.RLock()
	members := make([]*Instance, 0, len(webSocketChannels.members[channel]))
	for member := range webSocketChannels.members[channel] {
		if member != exclude {
			members = append(members, member)
		}
	}
	webSocketChannels.RUnlock()

	delivered := 0
	for _, member := range members {
		sender, ok := member.Fields["_sender"].(func(interface{}) error)
		if !ok || sender(message) != nil {
			unsubscribeWebSocketChannel(channel, member)
			continue
		}
		delivered++
	}
	return delivered
}

func webSocketChannelSubscriberCount(channel string) int {
	webSocketChannels.RLock()
	defer webSocketChannels.RUnlock()
	return len(webSocketChannels.members[channel])
}
