package native

// emitNativeServer emits the standalone AArch64 native HTTP server (fn_Server_start)
// supporting Linux AArch64 and Darwin ARM64 POSIX syscalls.
func (b *ARM64Builder) emitNativeServer() {
	const (
		httpHeaderHTML  = "HTTP/1.1 200 OK\r\nContent-Type: text/html; charset=utf-8\r\nCache-Control: no-cache, no-store, must-revalidate\r\nPragma: no-cache\r\nServer: Joss-Native-AOT/1.0\r\nConnection: close\r\n\r\n"
		httpHeaderJSON  = "HTTP/1.1 200 OK\r\nContent-Type: application/json; charset=utf-8\r\nServer: Joss-Native-AOT/1.0\r\nConnection: close\r\n\r\n"
		defaultHTMLBody = "<!DOCTYPE html><html lang=\"es\"><head><meta charset=\"utf-8\"><title>JosSecurity — Native AOT</title></head><body><h1>JosSecurity — Native AOT</h1><p>ONLINE (HTTP 200 OK)</p></body></html>"
		healthResponse  = "HTTP/1.1 200 OK\r\nContent-Type: text/plain; charset=utf-8\r\nConnection: close\r\n\r\nhealthy"
	)

	hasCompiledRouter := false
	if b.prog != nil {
		for _, fn := range b.prog.Functions {
			if fn.Name == "__joss_http_dispatch" {
				hasCompiledRouter = true
				break
			}
		}
	}

	var (
		sysSocket     uint64 = 198
		sysSetsockopt uint64 = 208
		sysBind       uint64 = 200
		sysListen     uint64 = 201
		sysAccept     uint64 = 202
		sysRead       uint64 = 63
		sysWrite      uint64 = 64
		sysClose      uint64 = 57
		sysOpenat     uint64 = 56
		sysExit       uint64 = 93
	)
	if b.target.OS == "darwin" {
		sysSocket = 0x2000061
		sysSetsockopt = 0x2000069
		sysBind = 0x2000068
		sysListen = 0x200006A
		sysAccept = 0x200001E
		sysRead = 0x2000003
		sysWrite = 0x2000004
		sysClose = 0x2000006
		sysOpenat = 0x20001CE
		sysExit = 0x2000001
	}

	emitSyscallNum := func(num uint64) {
		if b.target.OS == "darwin" {
			b.emitLoadImm64(regX16, num)
			b.emitSyscall()
		} else {
			b.emitLoadImm64(regX8, num)
			b.emitSyscall()
		}
	}

	b.defineLabel("fn_Server_start")
	b.emitProlog(128)

	// Save callee-saved registers
	b.emitStr(regX19, regX29, 16)
	b.emitStr(regX20, regX29, 24)
	b.emitStr(regX21, regX29, 32)
	b.emitStr(regX22, regX29, 40)
	b.emitStr(regX23, regX29, 48)
	b.emitStr(regX24, regX29, 56)

	// Allocate 16384-byte request buffer into x21
	b.emitLoadImm64(regX0, 16384)
	b.emitCall("__joss_os_alloc")
	b.emitMovReg(regX21, regX0) // x21 = reqBuf

	// Default port: 8000
	b.emitLoadImm64(regX22, 8000)

	// Try reading env.joss via openat(AT_FDCWD=-100, "env.joss", O_RDONLY=0, 0)
	b.emitLoadImm64(regX0, 0xFFFFFFFFFFFFFF9C)
	b.emitLoadStringAddr(regX1, "env.joss")
	b.emitLoadImm64(regX2, 0)
	b.emitLoadImm64(regX3, 0)
	emitSyscallNum(sysOpenat)
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condGE, ".srv_read_env_fd")

	// Try reading .env
	b.emitLoadImm64(regX0, 0xFFFFFFFFFFFFFF9C)
	b.emitLoadStringAddr(regX1, ".env")
	b.emitLoadImm64(regX2, 0)
	b.emitLoadImm64(regX3, 0)
	emitSyscallNum(sysOpenat)
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condGE, ".srv_read_env_fd")

	// Try reading /proc/self/environ
	b.emitLoadImm64(regX0, 0xFFFFFFFFFFFFFF9C)
	b.emitLoadStringAddr(regX1, "/proc/self/environ")
	b.emitLoadImm64(regX2, 0)
	b.emitLoadImm64(regX3, 0)
	emitSyscallNum(sysOpenat)
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condLT, ".srv_port_ready")

	b.defineLabel(".srv_read_env_fd")
	b.emitMovReg(regX9, regX0) // x9 = fd

	// read(fd, reqBuf, 4096)
	b.emitMovReg(regX0, regX9)
	b.emitMovReg(regX1, regX21)
	b.emitLoadImm64(regX2, 4096)
	emitSyscallNum(sysRead)
	b.emitMovReg(regX10, regX0) // x10 = bytes read

	// close(fd)
	b.emitMovReg(regX0, regX9)
	emitSyscallNum(sysClose)

	b.emitCmp(regX10, regXZR)
	b.emitBranchCond(condLE, ".srv_port_ready")

	// Null-terminate buffer at reqBuf + x10
	b.emitAdd(regX1, regX21, regX10)
	b.emitStrb(regXZR, regX1, 0)

	// Scan buffer for "PORT="
	b.emitMovReg(regX1, regX21)
	b.emitAdd(regX2, regX21, regX10)

	b.defineLabel(".srv_env_scan_loop")
	b.emitCmp(regX1, regX2)
	b.emitBranchCond(condGE, ".srv_port_ready")

	// Check "PORT="
	b.emitLdrb(regX0, regX1, 0)
	b.emitCmpImm(regX0, 80) // 'P'
	b.emitBranchCond(condNE, ".srv_env_scan_next")

	b.emitLdrb(regX0, regX1, 1)
	b.emitCmpImm(regX0, 79) // 'O'
	b.emitBranchCond(condNE, ".srv_env_scan_next")

	b.emitLdrb(regX0, regX1, 2)
	b.emitCmpImm(regX0, 82) // 'R'
	b.emitBranchCond(condNE, ".srv_env_scan_next")

	b.emitLdrb(regX0, regX1, 3)
	b.emitCmpImm(regX0, 84) // 'T'
	b.emitBranchCond(condNE, ".srv_env_scan_next")

	b.emitLdrb(regX0, regX1, 4)
	b.emitCmpImm(regX0, 61) // '='
	b.emitBranchCond(condNE, ".srv_env_scan_next")

	// Match! Skip "PORT="
	b.emitAddImm(regX1, regX1, 5)

	// Skip quote if present ('"' or '\'')
	b.emitLdrb(regX0, regX1, 0)
	b.emitCmpImm(regX0, 34) // '"'
	b.emitBranchCond(condEQ, ".srv_skip_quote")
	b.emitCmpImm(regX0, 39) // '\''
	b.emitBranchCond(condNE, ".srv_parse_port_digits")
	b.defineLabel(".srv_skip_quote")
	b.emitAddImm(regX1, regX1, 1)

	// Parse digits
	b.defineLabel(".srv_parse_port_digits")
	b.emitLoadImm64(regX3, 0)

	b.defineLabel(".srv_port_digit_loop")
	b.emitLdrb(regX0, regX1, 0)
	b.emitCmpImm(regX0, 48) // '0'
	b.emitBranchCond(condLT, ".srv_port_digits_done")
	b.emitCmpImm(regX0, 57) // '9'
	b.emitBranchCond(condGT, ".srv_port_digits_done")

	b.emitSubImm(regX0, regX0, 48)
	b.emitLoadImm64(regX4, 10)
	b.emitMul(regX3, regX3, regX4)
	b.emitAdd(regX3, regX3, regX0)

	b.emitAddImm(regX1, regX1, 1)
	b.emitBranch(".srv_port_digit_loop")

	b.defineLabel(".srv_port_digits_done")
	b.emitCmp(regX3, regXZR)
	b.emitBranchCond(condLE, ".srv_port_ready")
	b.emitLoadImm64(regX4, 65536)
	b.emitCmp(regX3, regX4)
	b.emitBranchCond(condGE, ".srv_port_ready")
	b.emitMovReg(regX22, regX3)
	b.emitBranch(".srv_port_ready")

	b.defineLabel(".srv_env_scan_next")
	b.emitAddImm(regX1, regX1, 1)
	b.emitBranch(".srv_env_scan_loop")

	b.defineLabel(".srv_port_ready")

	// socket(AF_INET=2, SOCK_STREAM=1, 0)
	b.emitLoadImm64(regX0, 2)
	b.emitLoadImm64(regX1, 1)
	b.emitLoadImm64(regX2, 0)
	emitSyscallNum(sysSocket)
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condLT, ".srv_exit_fail")
	b.emitMovReg(regX19, regX0) // x19 = listenSock

	// setsockopt(listenSock, SOL_SOCKET=1, SO_REUSEADDR=2, &optval, 4)
	b.emitLoadImm64(regX0, 1)
	b.emitStr(regX0, regX29, 80) // [x29, #80] = 1 (optval)
	b.emitMovReg(regX0, regX19)
	b.emitLoadImm64(regX1, 1) // SOL_SOCKET
	b.emitLoadImm64(regX2, 2) // SO_REUSEADDR
	b.emitAddImm(regX3, regX29, 80)
	b.emitLoadImm64(regX4, 4)
	emitSyscallNum(sysSetsockopt)

	// Prepare sockaddr_in at [x29, #64]
	b.emitStr(regXZR, regX29, 64)
	b.emitStr(regXZR, regX29, 72)

	// htons(x22)
	b.emitLoadImm64(regX3, 256)
	b.emitSDiv(regX4, regX22, regX3) // high_byte
	b.emitMul(regX5, regX4, regX3)
	b.emitSub(regX6, regX22, regX5) // low_byte

	b.emitLoadImm64(regX0, 2) // AF_INET
	b.emitStrb(regX0, regX29, 64)
	b.emitStrb(regXZR, regX29, 65)
	b.emitStrb(regX4, regX29, 66) // sin_port high_byte (big-endian)
	b.emitStrb(regX6, regX29, 67) // sin_port low_byte (big-endian)

	// bind(listenSock, &sockaddr_in, 16)
	b.emitMovReg(regX0, regX19)
	b.emitAddImm(regX1, regX29, 64)
	b.emitLoadImm64(regX2, 16)
	emitSyscallNum(sysBind)
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condLT, ".srv_bind_failed")
	b.emitBranch(".srv_listen_start")

	b.defineLabel(".srv_bind_failed")
	b.emitLoadImm64(regX3, 8000)
	b.emitCmp(regX22, regX3)
	b.emitBranchCond(condEQ, ".srv_exit_fail")

	// Close old socket that failed to bind before creating a new one
	b.emitMovReg(regX0, regX19)
	emitSyscallNum(sysClose)

	// Create fresh socket for fallback port 8000
	b.emitLoadImm64(regX0, 2)
	b.emitLoadImm64(regX1, 1)
	b.emitLoadImm64(regX2, 0)
	emitSyscallNum(sysSocket)
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condLT, ".srv_exit_fail")
	b.emitMovReg(regX19, regX0) // x19 = listenSock

	// setsockopt(listenSock, SOL_SOCKET=1, SO_REUSEADDR=2, &optval, 4)
	b.emitMovReg(regX0, regX19)
	b.emitLoadImm64(regX1, 1)
	b.emitLoadImm64(regX2, 2)
	b.emitAddImm(regX3, regX29, 80)
	b.emitLoadImm64(regX4, 4)
	emitSyscallNum(sysSetsockopt)

	b.emitLoadImm64(regX22, 8000)
	b.emitLoadImm64(regX0, 0x1F)
	b.emitStrb(regX0, regX29, 66)
	b.emitLoadImm64(regX0, 0x40)
	b.emitStrb(regX0, regX29, 67)
	b.emitMovReg(regX0, regX19)
	b.emitAddImm(regX1, regX29, 64)
	b.emitLoadImm64(regX2, 16)
	emitSyscallNum(sysBind)
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condLT, ".srv_exit_fail")

	b.defineLabel(".srv_listen_start")
	// listen(listenSock, 128)
	b.emitMovReg(regX0, regX19)
	b.emitLoadImm64(regX1, 128)
	emitSyscallNum(sysListen)
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condLT, ".srv_exit_fail")

	// Print: "[Joss Native Server AArch64] Escuchando en http://localhost:"
	b.emitLoadStringAddr(regX1, "[Joss Native Server AArch64] Escuchando en http://localhost:")
	b.emitMovReg(regX0, regX1)
	b.emitCall("__joss_rt_strlen")
	b.emitMovReg(regX2, regX0)
	b.emitLoadImm64(regX0, 1)
	b.emitCall("__joss_os_write")

	// Print port number
	b.emitMovReg(regX0, regX22)
	b.emitCall("joss_print_i64")

	// Print " (Ctrl+C para detener)"
	b.emitLoadStringAddr(regX0, " (Ctrl+C para detener)")
	b.emitCall("joss_print_string")

	// Accept loop
	b.defineLabel(".srv_accept_loop")
	b.emitMovReg(regX0, regX19)
	b.emitLoadImm64(regX1, 0)
	b.emitLoadImm64(regX2, 0)
	emitSyscallNum(sysAccept)
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condLT, ".srv_accept_loop")
	b.emitMovReg(regX20, regX0) // x20 = clientSock

	// read(clientSock, reqBuf, 16380)
	b.emitMovReg(regX0, regX20)
	b.emitMovReg(regX1, regX21)
	b.emitLoadImm64(regX2, 16380)
	emitSyscallNum(sysRead)
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condLE, ".srv_close_client")

	// Null-terminate request
	b.emitAdd(regX1, regX21, regX0)
	b.emitStrb(regXZR, regX1, 0)

	// Scan for first space
	b.emitMovReg(regX1, regX21)
	b.defineLabel(".srv_space_loop")
	b.emitLdrb(regX0, regX1, 0)
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condEQ, ".srv_default_path")
	b.emitCmpImm(regX0, 32)
	b.emitBranchCond(condEQ, ".srv_found_space")
	b.emitAddImm(regX1, regX1, 1)
	b.emitBranch(".srv_space_loop")

	b.defineLabel(".srv_found_space")
	b.emitAddImm(regX1, regX1, 1)
	b.emitMovReg(regX23, regX1) // x23 = path

	// Scan for end of path
	b.defineLabel(".srv_path_term_loop")
	b.emitLdrb(regX0, regX1, 0)
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condEQ, ".srv_path_terminated")
	b.emitCmpImm(regX0, 32)
	b.emitBranchCond(condEQ, ".srv_term_char")
	b.emitCmpImm(regX0, 63) // '?'
	b.emitBranchCond(condEQ, ".srv_term_char")
	b.emitCmpImm(regX0, 13) // '\r'
	b.emitBranchCond(condEQ, ".srv_term_char")
	b.emitCmpImm(regX0, 10) // '\n'
	b.emitBranchCond(condEQ, ".srv_term_char")
	b.emitAddImm(regX1, regX1, 1)
	b.emitBranch(".srv_path_term_loop")

	b.defineLabel(".srv_term_char")
	b.emitStrb(regXZR, regX1, 0)
	b.emitBranch(".srv_have_path")

	b.defineLabel(".srv_default_path")
	b.emitLoadStringAddr(regX23, "/")

	b.defineLabel(".srv_path_terminated")
	b.defineLabel(".srv_have_path")

	// Healthcheck route
	b.emitLoadStringAddr(regX0, "/__joss_health")
	b.emitMovReg(regX1, regX23)
	b.emitCall("__joss_rt_strcmp")
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condEQ, ".srv_serve_health")

	if hasCompiledRouter {
		b.emitMovReg(regX0, regX23)
		b.emitCall("fn___joss_http_dispatch")
		b.emitMovReg(regX24, regX0) // x24 = responseBody
		b.emitCmp(regX24, regXZR)
		b.emitBranchCond(condNE, ".srv_check_empty_body")
		b.emitLoadStringAddr(regX24, defaultHTMLBody)
		b.emitBranch(".srv_send_response")
		b.defineLabel(".srv_check_empty_body")
		b.emitLdrb(regX0, regX24, 0)
		b.emitCmp(regX0, regXZR)
		b.emitBranchCond(condNE, ".srv_send_response")
		b.emitLoadStringAddr(regX24, defaultHTMLBody)
	} else {
		b.emitLoadStringAddr(regX24, defaultHTMLBody)
	}

	b.defineLabel(".srv_send_response")
	// JSON vs HTML header
	b.emitLdrb(regX0, regX24, 0)
	b.emitCmpImm(regX0, 123) // '{'
	b.emitBranchCond(condEQ, ".srv_use_json_hdr")
	b.emitCmpImm(regX0, 91) // '['
	b.emitBranchCond(condEQ, ".srv_use_json_hdr")
	b.emitLoadStringAddr(regX1, httpHeaderHTML)
	b.emitBranch(".srv_write_hdr")

	b.defineLabel(".srv_use_json_hdr")
	b.emitLoadStringAddr(regX1, httpHeaderJSON)

	b.defineLabel(".srv_write_hdr")
	b.emitMovReg(regX9, regX1)
	b.emitMovReg(regX0, regX1)
	b.emitCall("__joss_rt_strlen")
	b.emitMovReg(regX2, regX0)
	b.emitMovReg(regX1, regX9)
	b.emitMovReg(regX0, regX20)
	emitSyscallNum(sysWrite)

	// Write response body
	b.emitMovReg(regX0, regX24)
	b.emitCall("__joss_rt_strlen")
	b.emitMovReg(regX2, regX0)
	b.emitMovReg(regX1, regX24)
	b.emitMovReg(regX0, regX20)
	emitSyscallNum(sysWrite)
	b.emitBranch(".srv_close_client")

	b.defineLabel(".srv_serve_health")
	b.emitLoadStringAddr(regX1, healthResponse)
	b.emitMovReg(regX9, regX1)
	b.emitMovReg(regX0, regX1)
	b.emitCall("__joss_rt_strlen")
	b.emitMovReg(regX2, regX0)
	b.emitMovReg(regX1, regX9)
	b.emitMovReg(regX0, regX20)
	emitSyscallNum(sysWrite)

	b.defineLabel(".srv_close_client")
	b.emitMovReg(regX0, regX20)
	emitSyscallNum(sysClose)
	b.emitBranch(".srv_accept_loop")

	b.defineLabel(".srv_exit_fail")
	b.emitLoadStringAddr(regX1, "[Joss Native Server AArch64] ERROR: No se pudo enlazar el socket TCP. Verifique permisos (puerto < 1024 requiere root o cap_net_bind_service).\n")
	b.emitMovReg(regX0, regX1)
	b.emitCall("__joss_rt_strlen")
	b.emitMovReg(regX2, regX0)
	b.emitLoadImm64(regX0, 2) // stderr fd = 2
	b.emitCall("__joss_os_write")
	b.emitLoadImm64(regX0, 1)
	emitSyscallNum(sysExit)

	b.defineLabel(".srv_exit_ok")
	b.emitLdr(regX19, regX29, 16)
	b.emitLdr(regX20, regX29, 24)
	b.emitLdr(regX21, regX29, 32)
	b.emitLdr(regX22, regX29, 40)
	b.emitLdr(regX23, regX29, 48)
	b.emitLdr(regX24, regX29, 56)
	b.emitEpilog(128)
}
