package core

import (
	"fmt"
)

// WindowState almacena las propiedades y manejadores de una ventana nativa de Joss en ejecución.
type WindowState struct {
	Title    string
	Width    int
	Height   int
	Visible  bool
	Handlers map[string]interface{}
}

func (r *Runtime) executeWindowMethod(instance *Instance, method string, args []interface{}) interface{} {
	if instance == nil {
		instance = &Instance{
			Class:  r.Classes["Window"],
			Fields: make(map[string]interface{}),
		}
	}

	state, ok := instance.Fields["_state"].(*WindowState)
	if !ok || state == nil {
		state = &WindowState{
			Title:    "Joss Window",
			Width:    1024,
			Height:   768,
			Visible:  false,
			Handlers: make(map[string]interface{}),
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
			if w, ok := args[1].(int); ok && w > 0 {
				width = w
			}
		}
		if len(args) > 2 {
			if h, ok := args[2].(int); ok && h > 0 {
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
			if w, ok := args[0].(int); ok {
				state.Width = w
			}
			if h, ok := args[1].(int); ok {
				state.Height = h
			}
		}
		return nil

	case "show":
		state.Visible = true
		return nil

	case "hide":
		state.Visible = false
		return nil

	case "close":
		state.Visible = false
		return nil

	case "pollEvents":
		// En modo headless o por defecto devuelve false (no hay más eventos pendientes)
		return false

	case "render":
		// El renderizado de vistas se despacha al árbol UI / GPU
		return nil

	case "on":
		if len(args) >= 2 && args[0] != nil {
			evt := fmt.Sprint(args[0])
			state.Handlers[evt] = args[1]
		}
		return nil

	default:
		return nil
	}
}
