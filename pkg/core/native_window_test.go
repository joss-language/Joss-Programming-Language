package core

import (
	"testing"
)

func TestWindowNativeClassExecution(t *testing.T) {
	code := `
		var $win = new Window()
		$win->open("Mi Ventana", 800, 600)
		$win->setTitle("Nuevo Titulo")
		$win->setSize(1280, 720)
		$win->show()
		$win->render("<div>Contenido</div>")
		$win->hide()
		$win->close()
	`

	executeCode(t, code)
}
