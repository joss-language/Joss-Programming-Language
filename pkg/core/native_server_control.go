package core

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// ServerStartCallback allows optional hook if needed
var ServerStartCallback func(fs http.FileSystem)

// Server Control Native Class
func (r *Runtime) executeServerControlMethod(instance *Instance, method string, args []interface{}) interface{} {
	switch method {
	case "start":
		fmt.Println("[Server::start] Iniciando servidor web...")
		if ServerStartCallback != nil {
			ServerStartCallback(GlobalFileSystem)
		} else {
			fmt.Println("Aviso: El servidor HTTP de Joss se ejecuta exclusivamente mediante el compilador nativo autónomo.")
			fmt.Println("Usa 'joss server start' o 'joss build' para compilar y ejecutar el servidor web nativo de alto rendimiento.")
		}
		return true

	case "spawn":
		// Server::spawn(name, command, port)
		if len(args) >= 3 {
			name := args[0].(string) // For future management
			commandStr := args[1].(string)
			port := args[2] // int or string or float

			// Security Check
			allow, ok := r.Env["ALLOW_SYSTEM_RUN"]
			if !ok || (allow != "true" && allow != "1") {
				fmt.Println("[Server::spawn] Error: Ejecución de servicios bloqueada. Configure ALLOW_SYSTEM_RUN=true en su entorno.")
				return false
			}

			// Validate Port
			portStr := fmt.Sprintf("%v", port)

			fmt.Printf("[Server::spawn] Iniciando servicio '%s' en puerto %s...\n", name, portStr)

			// Prepare Command
			parts := strings.Fields(commandStr)
			if len(parts) == 0 {
				return false
			}

			head := parts[0]
			tail := parts[1:]

			cmd := exec.Command(head, tail...)

			// Inject PORT env var
			cmd.Env = append(os.Environ(), "PORT="+portStr)

			// Pipe output to console for debugging
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			// Start in background
			err := cmd.Start()
			if err != nil {
				fmt.Printf("[Server::spawn] Error iniciando servicio: %v\n", err)
				return false
			}

			fmt.Printf("[Server::spawn] Servicio iniciado (PID: %d)\n", cmd.Process.Pid)

			return true
		}
		return false
	}
	return nil
}
