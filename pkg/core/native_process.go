package core

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
)

// Process Implementation
// Handles the "Process" native class for advanced external execution

func (r *Runtime) executeProcessMethod(instance *Instance, method string, args []interface{}) interface{} {
	// Retrieve internal state
	var cmd *exec.Cmd
	if c, ok := instance.Fields["_cmd"]; ok {
		cmd = c.(*exec.Cmd)
	}

	switch method {
	case "constructor":
		if err := r.RequireCapability("process"); err != nil {
			panic(err)
		}

		if len(args) == 0 {
			return instance
		}

		command := args[0].(string)
		var cmdArgs []string

		if len(args) > 1 {
			if list, ok := args[1].([]interface{}); ok {
				for _, arg := range list {
					cmdArgs = append(cmdArgs, fmt.Sprintf("%v", arg))
				}
			}
		}

		cmd = exec.Command(command, cmdArgs...)

		// Setup Pipes
		stmtIn, _ := cmd.StdinPipe()
		stmtOut, _ := cmd.StdoutPipe()
		stmtErr, _ := cmd.StderrPipe()

		instance.Fields["_cmd"] = cmd
		instance.Fields["_stdin"] = stmtIn
		instance.Fields["_stdout"] = stmtOut
		instance.Fields["_stderr"] = stmtErr
		instance.Fields["_started"] = false

		// Output Channels
		// We will create them when requested or start streaming immediately?
		// Let's create them on demand or start and fill buffers.
		// Better: use channels to stream lines.
		instance.Fields["_out_chan"] = r.makeChannel(100) // Buffered
		instance.Fields["_err_chan"] = r.makeChannel(100)

		return instance

	case "start":
		if cmd == nil {
			return false
		}
		if instance.Fields["_started"] == true {
			return false
		}

		// Windows specific: Hide window if option pending?
		// For now simple start.
		err := cmd.Start()
		if err != nil {
			fmt.Printf("[Process] Error initiating: %v\n", err)
			return false
		}

		instance.Fields["_started"] = true

		// Start background readers
		stdoutPipe := instance.Fields["_stdout"].(io.ReadCloser)
		stderrPipe := instance.Fields["_stderr"].(io.ReadCloser)
		outChan := instance.Fields["_out_chan"].(*Channel)
		errChan := instance.Fields["_err_chan"].(*Channel)

		// STDOUT Reader
		go func() {
			scanner := bufio.NewScanner(stdoutPipe)
			for scanner.Scan() {
				// Send line to channel
				// This might block if channel is full.
				// For async nature, we should be careful.
				// We use non-blocking send or large buffer?
				// Native channel send is not exposed easily here if we want to respect runtime locks?
				// Actually, channels in go are thread-safe.
				// r.Channel implementation uses Go channels.
				if err := outChan.TrySend(scanner.Text()); err != nil {
					break
				}
			}
			_ = outChan.TryClose()
		}()

		// STDERR Reader
		go func() {
			scanner := bufio.NewScanner(stderrPipe)
			for scanner.Scan() {
				if err := errChan.TrySend(scanner.Text()); err != nil {
					break
				}
			}
			_ = errChan.TryClose()
		}()

		return true

	case "wait":
		if cmd == nil {
			return -1
		}
		err := cmd.Wait()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return exitErr.ExitCode()
			}
			return -1
		}
		return 0 // Success

	case "kill":
		if cmd == nil || cmd.Process == nil {
			return false
		}
		// Try nice kill first?
		// On Windows Kill is forced.
		err := cmd.Process.Kill()
		return err == nil

	case "pid":
		if cmd == nil || cmd.Process == nil {
			return 0
		}
		return cmd.Process.Pid

	case "stdin":
		// Write to stdin
		if len(args) > 0 {
			input := fmt.Sprintf("%v", args[0])
			stdin := instance.Fields["_stdin"].(io.WriteCloser)
			io.WriteString(stdin, input+"\n") // Add newline? Maybe optional.
		}
		return instance

	case "stdout_chan":
		return instance.Fields["_out_chan"]

	case "stderr_chan":
		return instance.Fields["_err_chan"]
	}

	return nil
}

// Helper: Make Channel (Duplicate from runtime_utils or expose it?)
// Since Runtime struct is in same package 'core', we can access if makeChannel was a method.
// It is not. We should simulate it or add it to Runtime in runtime.go
// Ideally we should use the existing 'make_chan' logic.
// In runtime.go we have:
// func (r *Runtime) makeChannel(size int) *Channel { ... } (If it exists)
// Let's assume we need to add it or use manual creation.

// Looking at 'native.go' -> 'Stack' implementation...
// We can just create the struct manually if it's exported or same pkg.
// type Channel struct { Ch chan interface{} }
// It is defined in 'types.go' in same package 'core'.
func (r *Runtime) makeChannel(size int) *Channel {
	if size <= 0 {
		return &Channel{Ch: make(chan interface{})}
	}
	return &Channel{Ch: make(chan interface{}, size)}
}
