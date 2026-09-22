package core

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// executeFileStreamMethod handles methods on FileStream instances and static calls
func (r *Runtime) executeFileStreamMethod(instance *Instance, method string, args []interface{}) interface{} {
	switch method {
	case "open":
		path := ""
		mode := "r"
		if len(args) > 0 {
			path = fmt.Sprintf("%v", args[0])
		}
		if len(args) > 1 {
			mode = fmt.Sprintf("%v", args[1])
		}
		flag := os.O_RDONLY
		switch mode {
		case "w":
			flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		case "a":
			flag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
		case "r+":
			flag = os.O_RDWR
		case "w+":
			flag = os.O_RDWR | os.O_CREATE | os.O_TRUNC
		case "a+":
			flag = os.O_RDWR | os.O_CREATE | os.O_APPEND
		}
		file, err := os.OpenFile(path, flag, 0644)
		if err != nil {
			panic(&JossError{Type: "IOError", Message: fmt.Sprintf("FileStream::open failed: %v", err), File: r.CurrentFile})
		}
		inst := instance
		if inst == nil || inst.Class == nil || inst.Class.Name == nil || inst.Class.Name.Value != "FileStream" {
			inst = &Instance{Class: r.Classes["FileStream"], Fields: make(map[string]interface{})}
		}
		inst.Fields["_file"] = file
		inst.Fields["_path"] = path
		return inst

	case "read":
		file, ok := instance.Fields["_file"].(*os.File)
		if !ok || file == nil {
			panic(&JossError{Type: "IOError", Message: "FileStream is not open", File: r.CurrentFile})
		}
		length := int64(0)
		if len(args) > 0 {
			if n, ok := args[0].(int64); ok {
				length = n
			} else if n, ok := args[0].(int); ok {
				length = int64(n)
			}
		}
		if length <= 0 {
			content, err := io.ReadAll(file)
			if err != nil && err != io.EOF {
				panic(&JossError{Type: "IOError", Message: fmt.Sprintf("FileStream::read failed: %v", err), File: r.CurrentFile})
			}
			return string(content)
		}
		buf := make([]byte, length)
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			panic(&JossError{Type: "IOError", Message: fmt.Sprintf("FileStream::read failed: %v", err), File: r.CurrentFile})
		}
		return string(buf[:n])

	case "write":
		file, ok := instance.Fields["_file"].(*os.File)
		if !ok || file == nil {
			panic(&JossError{Type: "IOError", Message: "FileStream is not open", File: r.CurrentFile})
		}
		data := ""
		if len(args) > 0 {
			data = fmt.Sprintf("%v", args[0])
		}
		n, err := file.WriteString(data)
		if err != nil {
			panic(&JossError{Type: "IOError", Message: fmt.Sprintf("FileStream::write failed: %v", err), File: r.CurrentFile})
		}
		return int64(n)

	case "seek":
		file, ok := instance.Fields["_file"].(*os.File)
		if !ok || file == nil {
			panic(&JossError{Type: "IOError", Message: "FileStream is not open", File: r.CurrentFile})
		}
		offset := int64(0)
		whence := 0
		if len(args) > 0 {
			if n, ok := args[0].(int64); ok {
				offset = n
			} else if n, ok := args[0].(int); ok {
				offset = int64(n)
			}
		}
		if len(args) > 1 {
			if w, ok := args[1].(int64); ok {
				whence = int(w)
			} else if w, ok := args[1].(int); ok {
				whence = w
			}
		}
		pos, err := file.Seek(offset, whence)
		if err != nil {
			panic(&JossError{Type: "IOError", Message: fmt.Sprintf("FileStream::seek failed: %v", err), File: r.CurrentFile})
		}
		return pos

	case "size":
		file, ok := instance.Fields["_file"].(*os.File)
		if !ok || file == nil {
			return int64(0)
		}
		fi, err := file.Stat()
		if err != nil {
			return int64(0)
		}
		return fi.Size()

	case "flush":
		if file, ok := instance.Fields["_file"].(*os.File); ok && file != nil {
			_ = file.Sync()
		}
		return nil

	case "close":
		if file, ok := instance.Fields["_file"].(*os.File); ok && file != nil {
			_ = file.Close()
			instance.Fields["_file"] = nil
		}
		return nil
	}
	return nil
}

// executeStreamReaderMethod handles methods on StreamReader instances
func (r *Runtime) executeStreamReaderMethod(instance *Instance, method string, args []interface{}) interface{} {
	switch method {
	case "open":
		var file *os.File
		if len(args) > 0 {
			if streamInst, ok := args[0].(*Instance); ok {
				if f, ok := streamInst.Fields["_file"].(*os.File); ok {
					file = f
				}
			} else if path, ok := args[0].(string); ok {
				f, err := os.Open(path)
				if err != nil {
					panic(&JossError{Type: "IOError", Message: fmt.Sprintf("StreamReader::open failed: %v", err), File: r.CurrentFile})
				}
				file = f
				instance.Fields["_ownedFile"] = f
			}
		}
		if file == nil {
			panic(&JossError{Type: "IOError", Message: "StreamReader requires a valid FileStream or path", File: r.CurrentFile})
		}
		instance.Fields["_reader"] = bufio.NewReader(file)
		return instance

	case "readLine":
		reader, ok := instance.Fields["_reader"].(*bufio.Reader)
		if !ok || reader == nil {
			return nil
		}
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			return nil
		}
		// Strip trailing newline if present
		if len(line) > 0 && line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
		}
		return line

	case "readToEnd":
		reader, ok := instance.Fields["_reader"].(*bufio.Reader)
		if !ok || reader == nil {
			return ""
		}
		content, err := io.ReadAll(reader)
		if err != nil && err != io.EOF {
			panic(&JossError{Type: "IOError", Message: fmt.Sprintf("StreamReader::readToEnd failed: %v", err), File: r.CurrentFile})
		}
		return string(content)

	case "close":
		if file, ok := instance.Fields["_ownedFile"].(*os.File); ok && file != nil {
			_ = file.Close()
			instance.Fields["_ownedFile"] = nil
		}
		instance.Fields["_reader"] = nil
		return nil
	}
	return nil
}

// executeStreamWriterMethod handles methods on StreamWriter instances
func (r *Runtime) executeStreamWriterMethod(instance *Instance, method string, args []interface{}) interface{} {
	switch method {
	case "open":
		var file *os.File
		appendMode := false
		if len(args) > 1 {
			if b, ok := args[1].(bool); ok {
				appendMode = b
			}
		}
		if len(args) > 0 {
			if streamInst, ok := args[0].(*Instance); ok {
				if f, ok := streamInst.Fields["_file"].(*os.File); ok {
					file = f
				}
			} else if path, ok := args[0].(string); ok {
				flag := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
				if appendMode {
					flag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
				}
				f, err := os.OpenFile(path, flag, 0644)
				if err != nil {
					panic(&JossError{Type: "IOError", Message: fmt.Sprintf("StreamWriter::open failed: %v", err), File: r.CurrentFile})
				}
				file = f
				instance.Fields["_ownedFile"] = f
			}
		}
		if file == nil {
			panic(&JossError{Type: "IOError", Message: "StreamWriter requires a valid FileStream or path", File: r.CurrentFile})
		}
		instance.Fields["_writer"] = bufio.NewWriter(file)
		return instance

	case "write":
		writer, ok := instance.Fields["_writer"].(*bufio.Writer)
		if !ok || writer == nil {
			panic(&JossError{Type: "IOError", Message: "StreamWriter is not open", File: r.CurrentFile})
		}
		text := ""
		if len(args) > 0 {
			text = fmt.Sprintf("%v", args[0])
		}
		n, err := writer.WriteString(text)
		if err != nil {
			panic(&JossError{Type: "IOError", Message: fmt.Sprintf("StreamWriter::write failed: %v", err), File: r.CurrentFile})
		}
		return int64(n)

	case "writeLine":
		writer, ok := instance.Fields["_writer"].(*bufio.Writer)
		if !ok || writer == nil {
			panic(&JossError{Type: "IOError", Message: "StreamWriter is not open", File: r.CurrentFile})
		}
		text := ""
		if len(args) > 0 {
			text = fmt.Sprintf("%v", args[0])
		}
		n, err := writer.WriteString(text + "\n")
		if err != nil {
			panic(&JossError{Type: "IOError", Message: fmt.Sprintf("StreamWriter::writeLine failed: %v", err), File: r.CurrentFile})
		}
		return int64(n)

	case "flush":
		if writer, ok := instance.Fields["_writer"].(*bufio.Writer); ok && writer != nil {
			_ = writer.Flush()
		}
		return nil

	case "close":
		if writer, ok := instance.Fields["_writer"].(*bufio.Writer); ok && writer != nil {
			_ = writer.Flush()
			instance.Fields["_writer"] = nil
		}
		if file, ok := instance.Fields["_ownedFile"].(*os.File); ok && file != nil {
			_ = file.Close()
			instance.Fields["_ownedFile"] = nil
		}
		return nil
	}
	return nil
}

// executeStreamMethod handles SSE Server Stream methods
func (r *Runtime) executeStreamMethod(instance *Instance, method string, args []interface{}) interface{} {
	wVal, ok := instance.Fields["_writer"]
	if !ok {
		return nil
	}
	w, ok := wVal.(http.ResponseWriter)
	if !ok {
		return nil
	}

	switch method {
	case "send":
		var payload interface{}
		var eventType string

		if len(args) == 1 {
			payload = args[0]
		} else if len(args) >= 2 {
			if t, ok := args[0].(string); ok {
				eventType = t
			}
			payload = args[1]
		}

		var dataStr string
		if payloadMap, ok := payload.(map[string]interface{}); ok {
			bytes, _ := json.Marshal(payloadMap)
			dataStr = string(bytes)
		} else if payloadSlice, ok := payload.([]interface{}); ok {
			bytes, _ := json.Marshal(payloadSlice)
			dataStr = string(bytes)
		} else {
			dataStr = fmt.Sprintf("%v", payload)
		}

		if eventType != "" {
			fmt.Fprintf(w, "event: %s\n", eventType)
		}
		fmt.Fprintf(w, "data: %s\n\n", dataStr)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		return true

	case "close":
		fmt.Fprintf(w, "data: [DONE]\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		return true
	}
	return nil
}

// NewStreamInstance creates an instance of the SSE Stream native class
func NewStreamInstance(r *Runtime, w http.ResponseWriter) *Instance {
	if _, ok := r.Classes["Stream"]; !ok {
		return nil
	}
	return &Instance{
		Class: r.Classes["Stream"],
		Fields: map[string]interface{}{
			"_writer": w,
		},
	}
}
