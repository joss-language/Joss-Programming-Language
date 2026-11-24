package core

import (
	"fmt"
)

// WindowState almacena las propiedades y manejadores de una ventana nativa de Joss en ejecución.
type WindowState struct {
	Title       string
	Width       int
	Height      int
	MinWidth    int
	MinHeight   int
	Visible     bool
	Resizable   bool
	Frameless   bool
	AlwaysOnTop bool
	Maximized   bool
	Minimized   bool
	Theme       string
	URL         string
	HTML        string
	Handlers    map[string]interface{}
	Bindings    map[string]interface{}
}

func windowIntArg(v interface{}, fallback int) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return fallback
	}
}

func (r *Runtime) executeWindowMethod(instance *Instance, method string, args []interface{}) interface{} {
	if instance == nil || instance == r.Variables["Window"] {
		instance = &Instance{
			Class:  r.Classes["Window"],
			Fields: make(map[string]interface{}),
		}
	}

	state, ok := instance.Fields["_state"].(*WindowState)
	if !ok || state == nil {
		state = &WindowState{
			Title:     "Joss Window",
			Width:     1024,
			Height:    768,
			Visible:   false,
			Resizable: true,
			Theme:     "system",
			Handlers:  make(map[string]interface{}),
			Bindings:  make(map[string]interface{}),
		}
		instance.Fields["_state"] = state
	}

	switch method {
	case "open":
		title := "Joss Application"
		width := 1024
		height := 768
		if len(args) > 0 && args[0] != nil {
			title = fmt.Sprint(args[0])
		}
		if len(args) > 1 {
			if w := windowIntArg(args[1], 0); w > 0 {
				width = w
			}
		}
		if len(args) > 2 {
			if h := windowIntArg(args[2], 0); h > 0 {
				height = h
			}
		}
		state.Title = title
		state.Width = width
		state.Height = height
		state.Visible = true
		return instance

	case "setTitle":
		if len(args) > 0 && args[0] != nil {
			state.Title = fmt.Sprint(args[0])
		}
		return nil

	case "setSize":
		if len(args) >= 2 {
			if w := windowIntArg(args[0], 0); w > 0 {
				state.Width = w
			}
			if h := windowIntArg(args[1], 0); h > 0 {
				state.Height = h
			}
		}
		return nil

	case "setMinSize":
		if len(args) >= 2 {
			state.MinWidth = windowIntArg(args[0], 0)
			state.MinHeight = windowIntArg(args[1], 0)
		}
		return nil

	case "setResizable":
		if len(args) > 0 {
			state.Resizable = isTruthy(args[0])
		}
		return nil

	case "setFrameless":
		if len(args) > 0 {
			state.Frameless = isTruthy(args[0])
		}
		return nil

	case "setAlwaysOnTop":
		if len(args) > 0 {
			state.AlwaysOnTop = isTruthy(args[0])
		}
		return nil

	case "setTheme":
		if len(args) > 0 && args[0] != nil {
			state.Theme = fmt.Sprint(args[0])
		}
		return nil

	case "navigate":
		if len(args) > 0 && args[0] != nil {
			state.URL = fmt.Sprint(args[0])
		}
		return nil

	case "setHtml":
		if len(args) > 0 && args[0] != nil {
			state.HTML = fmt.Sprint(args[0])
		}
		return nil

	case "eval":
		return nil

	case "bind":
		if len(args) >= 2 && args[0] != nil {
			state.Bindings[fmt.Sprint(args[0])] = args[1]
		}
		return nil

	case "dispatch":
		if len(args) == 0 || args[0] == nil {
			return ""
		}
		name := fmt.Sprint(args[0])
		payload := ""
		if len(args) > 1 && args[1] != nil {
			payload = fmt.Sprint(args[1])
		}
		handler, exists := state.Bindings[name]
		if !exists || handler == nil {
			return ""
		}
		res := r.CallCallable(handler, []interface{}{payload})
		if res == nil {
			return ""
		}
		return fmt.Sprint(res)

	case "maximize":
		state.Maximized = true
		state.Minimized = false
		return nil

	case "minimize":
		state.Minimized = true
		state.Maximized = false
		return nil

	case "show":
		state.Visible = true
		return nil

	case "hide":
		state.Visible = false
		return nil

	case "close":
		state.Visible = false
		if h, ok := state.Handlers["close"]; ok && h != nil {
			_ = r.CallCallable(h, nil)
		}
		return nil

	case "run":
		return nil

	case "pollEvents":
		return false

	case "render":
		if len(args) > 0 && args[0] != nil {
			state.HTML = fmt.Sprint(args[0])
		}
		return nil

	case "on":
		if len(args) >= 2 && args[0] != nil {
			evt := fmt.Sprint(args[0])
			state.Handlers[evt] = args[1]
		}
		return nil

	case "getTitle":
		return state.Title

	case "getWidth":
		return int64(state.Width)

	case "getHeight":
		return int64(state.Height)

	case "isVisible":
		return state.Visible

	default:
		return nil
	}
}
