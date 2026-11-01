package native

// emitNativeServer emits the standalone x86_64 native HTTP server (fn_Server_start)
// supporting Windows Winsock2 and Linux/Darwin POSIX syscalls.
func (b *MachineCodeBuilder) emitNativeServer() {
	const (
		httpHeaderHTML  = "HTTP/1.1 200 OK\r\nContent-Type: text/html; charset=utf-8\r\nServer: Joss-Native-AOT/1.0\r\nConnection: close\r\n\r\n"
		httpHeaderCSS   = "HTTP/1.1 200 OK\r\nContent-Type: text/css; charset=utf-8\r\nServer: Joss-Native-AOT/1.0\r\nConnection: close\r\n\r\n"
		httpHeaderJS    = "HTTP/1.1 200 OK\r\nContent-Type: application/javascript; charset=utf-8\r\nServer: Joss-Native-AOT/1.0\r\nConnection: close\r\n\r\n"
		httpHeaderSVG   = "HTTP/1.1 200 OK\r\nContent-Type: image/svg+xml\r\nServer: Joss-Native-AOT/1.0\r\nConnection: close\r\n\r\n"
		httpHeaderPNG   = "HTTP/1.1 200 OK\r\nContent-Type: image/png\r\nServer: Joss-Native-AOT/1.0\r\nConnection: close\r\n\r\n"
		httpHeader404   = "HTTP/1.1 404 Not Found\r\nContent-Type: text/plain; charset=utf-8\r\nConnection: close\r\n\r\n404 Not Found"
		defaultHTMLBody = "<!DOCTYPE html><html lang=\"es\"><head><meta charset=\"utf-8\"><title>JosSecurity — Native AOT</title></head><body><h1>JosSecurity — Native AOT</h1><p>ONLINE (HTTP 200 OK)</p></body></html>"
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

	b.defineLabel("fn_Server_start")
	if b.target.OS == "windows" {
		b.emitBytes(0x55, 0x48, 0x89, 0xE5)                               // push rbp; mov rbp, rsp
		b.emitBytes(0x53, 0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push rbx, r12..r15 (40 bytes)
		b.emitBytes(0x48, 0x81, 0xEC, 0x98, 0x00, 0x00, 0x00)             // sub rsp, 152 (total 192 bytes, 16-byte aligned)

		// 1. Load .env / env.joss file dynamically into environment map
		b.emitCallLabel("__joss_load_env_file")

		// 2. Announce server listening
		b.emitLeaRCXString("🚀 [Joss Native Server] Escuchando en http://localhost:8000 (Ctrl+C para detener)")
		b.emitCallLabel("joss_print_string")

		// Check JOSS_NATIVE_CHECK_ONLY environment variable for non-blocking CI smoke checks
		b.emitLeaRCXString("kernel32.dll")
		b.emitCallIAT(IATLoadLibraryA)
		b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
		b.emitJeLabel(".srv_start_ws2")
		b.emitBytes(0x49, 0x89, 0xC7) // mov r15, rax (hKernel32)

		b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15
		b.emitLeaRDXString("GetEnvironmentVariableA")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
		b.emitJeLabel(".srv_start_ws2")
		b.emitBytes(0x49, 0x89, 0xC4) // mov r12, rax
		b.emitLeaRCXString("JOSS_NATIVE_CHECK_ONLY")
		b.emitBytes(0x48, 0x8D, 0x55, 0xA0)             // lea rdx, [rbp - 96]
		b.emitBytes(0x41, 0xB8, 0x10, 0x00, 0x00, 0x00) // mov r8d, 16
		b.emitBytes(0x41, 0xFF, 0xD4)                   // call r12
		b.emitBytes(0x85, 0xC0)                         // test eax, eax
		b.emitJneLabel(".srv_exit_ok")

		b.defineLabel(".srv_start_ws2")
		b.emitLeaRCXString("ws2_32.dll")
		b.emitCallIAT(IATLoadLibraryA)
		b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
		b.emitJeLabel(".srv_exit_ok")
		b.emitBytes(0x49, 0x89, 0xC4) // mov r12, rax (hWs2)

		// Allocate 16384-byte request buffer (also used as WSADATA buffer during WSAStartup)
		b.emitBytes(0xB9, 0x00, 0x40, 0x00, 0x00) // mov ecx, 16384
		b.emitCallLabel("__joss_os_alloc")
		b.emitBytes(0x48, 0x89, 0x45, 0xB0) // mov [rbp - 80], rax (reqBuf)

		// WSAStartup(0x0202, reqBuf)
		b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
		b.emitLeaRDXString("WSAStartup")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0xB9, 0x02, 0x02, 0x00, 0x00) // mov ecx, 0x0202
		b.emitBytes(0x48, 0x8B, 0x55, 0xB0)       // mov rdx, [rbp - 80]
		b.emitBytes(0xFF, 0xD0)                   // call rax

		// socket(AF_INET=2, SOCK_STREAM=1, IPPROTO_TCP=6)
		b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
		b.emitLeaRDXString("socket")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0xB9, 0x02, 0x00, 0x00, 0x00)       // mov ecx, 2
		b.emitBytes(0xBA, 0x01, 0x00, 0x00, 0x00)       // mov edx, 1
		b.emitBytes(0x41, 0xB8, 0x06, 0x00, 0x00, 0x00) // mov r8d, 6
		b.emitBytes(0xFF, 0xD0)                         // call rax
		b.emitBytes(0x49, 0x89, 0xC5)                   // mov r13, rax (listenSock)

		// setsockopt(listenSock, SOL_SOCKET=0xFFFF, SO_REUSEADDR=4, &optval, 4)
		b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
		b.emitLeaRDXString("setsockopt")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0xC7, 0x45, 0x98, 0x01, 0x00, 0x00, 0x00)             // mov dword ptr [rbp - 104], 1
		b.emitBytes(0x4C, 0x89, 0xE9)                                     // mov rcx, r13
		b.emitBytes(0xBA, 0xFF, 0xFF, 0x00, 0x00)                         // mov edx, 0xFFFF
		b.emitBytes(0x41, 0xB8, 0x04, 0x00, 0x00, 0x00)                   // mov r8d, 4
		b.emitBytes(0x4C, 0x8D, 0x4D, 0x98)                               // lea r9, [rbp - 104]
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x20, 0x04, 0x00, 0x00, 0x00) // mov qword ptr [rsp + 32], 4
		b.emitBytes(0xFF, 0xD0)                                           // call rax

		// sockaddr_in at [rbp - 96]: sin_family=AF_INET(2), sin_addr=0
		b.emitBytes(0xC7, 0x45, 0xA0, 0x02, 0x00, 0x1F, 0x40)       // default: 0x401F0002 (port 8000)
		b.emitBytes(0xC7, 0x45, 0xA4, 0x00, 0x00, 0x00, 0x00)       // sin_addr = INADDR_ANY (0)
		b.emitBytes(0x48, 0xC7, 0x45, 0xA8, 0x00, 0x00, 0x00, 0x00) // sin_zero = 0

		// 1. Check OS environment variable "PORT" (takes precedence over .env file)
		b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15 (hKernel32)
		b.emitLeaRDXString("GetEnvironmentVariableA")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
		b.emitJeLabel(".srv_check_env_map_port")
		b.emitBytes(0x49, 0x89, 0xC2) // mov r10, rax
		b.emitLeaRCXString("PORT")
		b.emitBytes(0x48, 0x8B, 0x55, 0xB0)             // mov rdx, [rbp - 80] (reqBuf)
		b.emitBytes(0x41, 0xB8, 0x10, 0x00, 0x00, 0x00) // mov r8d, 16
		b.emitBytes(0x41, 0xFF, 0xD2)                   // call r10
		b.emitBytes(0x85, 0xC0)                         // test eax, eax
		b.emitJleLabel(".srv_check_env_map_port")
		b.emitBytes(0x48, 0x8B, 0x4D, 0xB0)             // mov rcx, [rbp - 80] (reqBuf)
		b.emitJmpLabel(".srv_parse_port")

		b.defineLabel(".srv_check_env_map_port")
		// 2. Lookup PORT in envMap (dataOffEnvMap)
		b.emitMovRCXData(dataOffEnvMap)
		b.emitBytes(0x48, 0x85, 0xC9)
		b.emitJeLabel(".srv_port_done")
		b.emitLeaRDXString("PORT")
		b.emitCallLabel("joss_map_get_i64")
		b.emitBytes(0x48, 0x85, 0xC0)
		b.emitJeLabel(".srv_check_joss_port")
		b.emitBytes(0x48, 0x89, 0xC1) // rcx = portStr
		b.emitJmpLabel(".srv_parse_port")

		b.defineLabel(".srv_check_joss_port")
		b.emitMovRCXData(dataOffEnvMap)
		b.emitLeaRDXString("JOSS_PORT")
		b.emitCallLabel("joss_map_get_i64")
		b.emitBytes(0x48, 0x85, 0xC0)
		b.emitJeLabel(".srv_port_done")
		b.emitBytes(0x48, 0x89, 0xC1)

		// Parse port string (rcx) with mini atoi
		b.defineLabel(".srv_parse_port")
		b.emitBytes(0x31, 0xD2) // edx = portVal = 0
		b.defineLabel(".srv_port_loop")
		b.emitBytes(0x0F, 0xB6, 0x01) // movzx eax, byte ptr [rcx]
		b.emitBytes(0x83, 0xF8, 0x30) // cmp eax, '0'
		b.emitJlLabel(".srv_port_parsed")
		b.emitBytes(0x83, 0xF8, 0x39) // cmp eax, '9'
		b.emitJgLabel(".srv_port_parsed")
		b.emitBytes(0x83, 0xE8, 0x30)                   // sub eax, '0'
		b.emitBytes(0x69, 0xD2, 0x0A, 0x00, 0x00, 0x00) // imul edx, edx, 10
		b.emitBytes(0x01, 0xC2)                         // add edx, eax
		b.emitBytes(0x48, 0xFF, 0xC1)                   // inc rcx
		b.emitJmpLabel(".srv_port_loop")

		b.defineLabel(".srv_port_parsed")
		b.emitBytes(0x85, 0xD2) // test edx, edx
		b.emitJleLabel(".srv_port_done")
		b.emitBytes(0x81, 0xFA, 0xFF, 0xFF, 0x00, 0x00) // cmp edx, 65535
		b.emitJgLabel(".srv_port_done")
		// htons: exchange dl and dh
		b.emitBytes(0x86, 0xD6)             // xchg dl, dh
		b.emitBytes(0x66, 0x89, 0x55, 0xA2) // mov word ptr [rbp - 94], dx

		b.defineLabel(".srv_port_done")

		// bind(listenSock, &sockaddr_in, 16)
		b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
		b.emitLeaRDXString("bind")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x4C, 0x89, 0xE9)                   // mov rcx, r13
		b.emitBytes(0x48, 0x8D, 0x55, 0xA0)             // lea rdx, [rbp - 96]
		b.emitBytes(0x41, 0xB8, 0x10, 0x00, 0x00, 0x00) // mov r8d, 16
		b.emitBytes(0xFF, 0xD0)                         // call rax

		// listen(listenSock, 128)
		b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
		b.emitLeaRDXString("listen")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x4C, 0x89, 0xE9)             // mov rcx, r13
		b.emitBytes(0xBA, 0x80, 0x00, 0x00, 0x00) // mov edx, 128
		b.emitBytes(0xFF, 0xD0)                   // call rax

		// Resolve accept, recv, send, closesocket
		b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
		b.emitLeaRDXString("accept")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x48, 0x89, 0x45, 0xD0) // mov [rbp - 48], rax (pAccept)

		b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
		b.emitLeaRDXString("recv")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x48, 0x89, 0x45, 0xC8) // mov [rbp - 56], rax (pRecv)

		b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
		b.emitLeaRDXString("send")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x48, 0x89, 0x45, 0xC0) // mov [rbp - 64], rax (pSend)

		b.emitBytes(0x4C, 0x89, 0xE1) // mov rcx, r12
		b.emitLeaRDXString("closesocket")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x48, 0x89, 0x45, 0xB8) // mov [rbp - 72], rax (pClosesocket)

		// Resolve CreateFileA, GetFileSize, ReadFile, CloseHandle from kernel32.dll (in r15)
		b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15
		b.emitLeaRDXString("CreateFileA")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x48, 0x89, 0x45, 0xA8) // mov [rbp - 88], rax (pCreateFileA)

		b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15
		b.emitLeaRDXString("GetFileSize")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x48, 0x89, 0x45, 0xA0) // mov [rbp - 96], rax (pGetFileSize)

		b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15
		b.emitLeaRDXString("ReadFile")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x48, 0x89, 0x45, 0x98) // mov [rbp - 104], rax (pReadFile)

		b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15
		b.emitLeaRDXString("CloseHandle")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x48, 0x89, 0x45, 0x90) // mov [rbp - 112], rax (pCloseHandle)

		// HTTP Accept Loop
		b.defineLabel(".srv_accept_loop")
		b.emitBytes(0x4C, 0x89, 0xE9)       // mov rcx, r13 (listenSock)
		b.emitBytes(0x31, 0xD2)             // xor edx, edx
		b.emitBytes(0x45, 0x31, 0xC0)       // xor r8d, r8d
		b.emitBytes(0x48, 0x8B, 0x45, 0xD0) // mov rax, [rbp - 48] (pAccept)
		b.emitBytes(0xFF, 0xD0)             // call rax
		b.emitBytes(0x48, 0x83, 0xF8, 0xFF) // cmp rax, -1
		b.emitJeLabel(".srv_exit_ok")
		b.emitBytes(0x49, 0x89, 0xC6) // mov r14, rax (clientSock)

		// recv(clientSock, reqBuf, 16380, 0)
		b.emitBytes(0x4C, 0x89, 0xF1)                   // mov rcx, r14
		b.emitBytes(0x48, 0x8B, 0x55, 0xB0)             // mov rdx, [rbp - 80] (reqBuf)
		b.emitBytes(0x41, 0xB8, 0xFC, 0x3F, 0x00, 0x00) // mov r8d, 16380
		b.emitBytes(0x45, 0x31, 0xC9)                   // xor r9d, r9d
		b.emitBytes(0x48, 0x8B, 0x45, 0xC8)             // mov rax, [rbp - 56] (pRecv)
		b.emitBytes(0xFF, 0xD0)                         // call rax
		b.emitBytes(0x85, 0xC0)                         // test eax, eax
		b.emitJleLabel(".srv_close_client")

		// Null-terminate received request at reqBuf[eax]
		b.emitBytes(0x48, 0x98)             // cdqe
		b.emitBytes(0x48, 0x8B, 0x55, 0xB0) // mov rdx, [rbp - 80]
		b.emitBytes(0xC6, 0x04, 0x02, 0x00) // mov byte ptr [rdx + rax], 0

		// Extract HTTP request path from first line: "GET /path?query HTTP/1.1"
		b.emitBytes(0x49, 0x89, 0xD7) // mov r15, rdx (scan ptr)
		b.defineLabel(".srv_find_space1")
		b.emitBytes(0x41, 0x0F, 0xB6, 0x07) // movzx eax, byte ptr [r15]
		b.emitBytes(0x85, 0xC0)             // test eax, eax
		b.emitJeLabel(".srv_default_path")
		b.emitBytes(0x83, 0xF8, 0x20) // cmp eax, ' '
		b.emitJeLabel(".srv_found_space1")
		b.emitBytes(0x49, 0xFF, 0xC7) // inc r15
		b.emitJmpLabel(".srv_find_space1")

		b.defineLabel(".srv_found_space1")
		b.emitBytes(0x49, 0xFF, 0xC7) // inc r15 (r15 = start of path)
		b.emitBytes(0x4C, 0x89, 0xFB) // mov rbx, r15 (rbx = cursor for end of path)
		b.defineLabel(".srv_find_path_end")
		b.emitBytes(0x0F, 0xB6, 0x03) // movzx eax, byte ptr [rbx]
		b.emitBytes(0x85, 0xC0)       // test eax, eax
		b.emitJeLabel(".srv_terminate_path")
		b.emitBytes(0x83, 0xF8, 0x20) // cmp eax, ' '
		b.emitJeLabel(".srv_terminate_path")
		b.emitBytes(0x83, 0xF8, 0x3F) // cmp eax, '?'
		b.emitJeLabel(".srv_terminate_path")
		b.emitBytes(0x83, 0xF8, 0x0D) // cmp eax, '\r'
		b.emitJeLabel(".srv_terminate_path")
		b.emitBytes(0x83, 0xF8, 0x0A) // cmp eax, '\n'
		b.emitJeLabel(".srv_terminate_path")
		b.emitBytes(0x48, 0xFF, 0xC3) // inc rbx
		b.emitJmpLabel(".srv_find_path_end")

		b.defineLabel(".srv_terminate_path")
		b.emitBytes(0xC6, 0x03, 0x00) // mov byte ptr [rbx], 0
		b.emitJmpLabel(".srv_have_path")

		b.defineLabel(".srv_default_path")
		b.emitLeaRCXString("/")
		b.emitBytes(0x49, 0x89, 0xCF) // mov r15, rcx

		b.defineLabel(".srv_have_path")
		// Check if path starts with "/public/" ("/pub" + 'l') or "/assets/" ("/ass" + 'e')
		b.emitBytes(0x41, 0x81, 0x3F, 0x2F, 0x70, 0x75, 0x62) // cmp dword ptr [r15], 0x6275702F ("/pub")
		b.emitJneLabel(".srv_check_assets")
		b.emitBytes(0x41, 0x80, 0x7F, 0x04, 0x6C) // cmp byte ptr [r15 + 4], 'l'
		b.emitJeLabel(".srv_serve_static")

		b.defineLabel(".srv_check_assets")
		b.emitBytes(0x41, 0x81, 0x3F, 0x2F, 0x61, 0x73, 0x73) // cmp dword ptr [r15], 0x7373612F ("/ass")
		b.emitJneLabel(".srv_dispatch_route")
		b.emitBytes(0x41, 0x80, 0x7F, 0x04, 0x65) // cmp byte ptr [r15 + 4], 'e'
		b.emitJneLabel(".srv_dispatch_route")

		b.defineLabel(".srv_serve_static")
		// Static file path relative to working directory is r15 + 1 (skip leading '/')
		b.emitBytes(0x49, 0x8D, 0x4F, 0x01)                               // lea rcx, [r15 + 1]
		b.emitBytes(0xBA, 0x00, 0x00, 0x00, 0x80)                         // mov edx, 0x80000000 (GENERIC_READ)
		b.emitBytes(0x41, 0xB8, 0x01, 0x00, 0x00, 0x00)                   // mov r8d, 1 (FILE_SHARE_READ)
		b.emitBytes(0x45, 0x31, 0xC9)                                     // xor r9d, r9d (NULL)
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x20, 0x03, 0x00, 0x00, 0x00) // mov qword ptr [rsp + 32], 3 (OPEN_EXISTING)
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x28, 0x80, 0x00, 0x00, 0x00) // mov qword ptr [rsp + 40], 0x80 (FILE_ATTRIBUTE_NORMAL)
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x30, 0x00, 0x00, 0x00, 0x00) // mov qword ptr [rsp + 48], 0
		b.emitBytes(0x48, 0x8B, 0x45, 0xA8)                               // mov rax, [rbp - 88] (pCreateFileA)
		b.emitBytes(0xFF, 0xD0)                                           // call rax
		b.emitBytes(0x48, 0x83, 0xF8, 0xFF)                               // cmp rax, -1 (INVALID_HANDLE_VALUE)
		b.emitJeLabel(".srv_send_404")
		b.emitBytes(0x48, 0x89, 0xC3) // mov rbx, rax (hFile)

		// GetFileSize(hFile, NULL)
		b.emitBytes(0x48, 0x89, 0xD9)       // mov rcx, rbx
		b.emitBytes(0x31, 0xD2)             // xor edx, edx
		b.emitBytes(0x48, 0x8B, 0x45, 0xA0) // mov rax, [rbp - 96] (pGetFileSize)
		b.emitBytes(0xFF, 0xD0)             // call rax
		b.emitBytes(0x48, 0x89, 0x45, 0x88) // mov [rbp - 120], rax (fileSize)

		// Allocate buffer for file content (fileSize + 16)
		b.emitBytes(0x48, 0x8D, 0x48, 0x10) // lea rcx, [rax + 16]
		b.emitCallLabel("__joss_os_alloc")
		b.emitBytes(0x48, 0x89, 0x45, 0x80) // mov [rbp - 128], rax (fileBuf)

		// ReadFile(hFile, fileBuf, fileSize, &bytesRead, NULL)
		b.emitBytes(0x48, 0x89, 0xD9)                                     // mov rcx, rbx (hFile)
		b.emitBytes(0x48, 0x8B, 0x55, 0x80)                               // mov rdx, [rbp - 128] (fileBuf)
		b.emitBytes(0x44, 0x8B, 0x45, 0x88)                               // mov r8d, dword ptr [rbp - 120] (fileSize)
		b.emitBytes(0x4C, 0x8D, 0x4D, 0x88)                               // lea r9, [rbp - 120] (&bytesRead)
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x20, 0x00, 0x00, 0x00, 0x00) // mov qword ptr [rsp + 32], 0
		b.emitBytes(0x48, 0x8B, 0x45, 0x98)                               // mov rax, [rbp - 104] (pReadFile)
		b.emitBytes(0xFF, 0xD0)                                           // call rax

		// CloseHandle(hFile)
		b.emitBytes(0x48, 0x89, 0xD9)       // mov rcx, rbx
		b.emitBytes(0x48, 0x8B, 0x45, 0x90) // mov rax, [rbp - 112] (pCloseHandle)
		b.emitBytes(0xFF, 0xD0)             // call rax

		// Choose MIME header based on file extension in r15
		b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15
		b.emitCallLabel("__joss_rt_strlen")
		b.emitBytes(0x48, 0x83, 0xF8, 0x04) // cmp rax, 4
		b.emitJlLabel(".srv_mime_html")
		b.emitBytes(0x41, 0x8B, 0x44, 0x07, 0xFC) // mov eax, dword ptr [r15 + rax - 4]
		b.emitBytes(0x3D, 0x2E, 0x63, 0x73, 0x73) // cmp eax, 0x7373632E (".css")
		b.emitJeLabel(".srv_mime_css")
		b.emitBytes(0x3D, 0x2E, 0x73, 0x76, 0x67) // cmp eax, 0x6776732E (".svg")
		b.emitJeLabel(".srv_mime_svg")
		b.emitBytes(0x3D, 0x2E, 0x70, 0x6E, 0x67) // cmp eax, 0x676E702E (".png")
		b.emitJeLabel(".srv_mime_png")
		b.emitBytes(0xC1, 0xE8, 0x08)             // shr eax, 8
		b.emitBytes(0x3D, 0x2E, 0x6A, 0x73, 0x00) // cmp eax, 0x00736A2E (".js")
		b.emitJeLabel(".srv_mime_js")

		b.defineLabel(".srv_mime_html")
		b.emitLeaRCXString(httpHeaderHTML)
		b.emitJmpLabel(".srv_send_static_hdr")
		b.defineLabel(".srv_mime_css")
		b.emitLeaRCXString(httpHeaderCSS)
		b.emitJmpLabel(".srv_send_static_hdr")
		b.defineLabel(".srv_mime_js")
		b.emitLeaRCXString(httpHeaderJS)
		b.emitJmpLabel(".srv_send_static_hdr")
		b.defineLabel(".srv_mime_svg")
		b.emitLeaRCXString(httpHeaderSVG)
		b.emitJmpLabel(".srv_send_static_hdr")
		b.defineLabel(".srv_mime_png")
		b.emitLeaRCXString(httpHeaderPNG)

		b.defineLabel(".srv_send_static_hdr")
		b.emitBytes(0x49, 0x89, 0xCF) // mov r15, rcx
		b.emitCallLabel("__joss_rt_strlen")
		b.emitBytes(0x49, 0x89, 0xC0)       // mov r8, rax
		b.emitBytes(0x4C, 0x89, 0xFA)       // mov rdx, r15
		b.emitBytes(0x4C, 0x89, 0xF1)       // mov rcx, r14 (clientSock)
		b.emitBytes(0x45, 0x31, 0xC9)       // xor r9d, r9d
		b.emitBytes(0x48, 0x8B, 0x45, 0xC0) // mov rax, [rbp - 64] (pSend)
		b.emitBytes(0xFF, 0xD0)             // call rax

		// Send static file body
		b.emitBytes(0x4C, 0x89, 0xF1)       // mov rcx, r14 (clientSock)
		b.emitBytes(0x48, 0x8B, 0x55, 0x80) // mov rdx, [rbp - 128] (fileBuf)
		b.emitBytes(0x44, 0x8B, 0x45, 0x88) // mov r8d, dword ptr [rbp - 120] (bytesRead)
		b.emitBytes(0x45, 0x31, 0xC9)       // xor r9d, r9d
		b.emitBytes(0x48, 0x8B, 0x45, 0xC0) // mov rax, [rbp - 64] (pSend)
		b.emitBytes(0xFF, 0xD0)             // call rax
		b.emitJmpLabel(".srv_close_client")

		b.defineLabel(".srv_send_404")
		b.emitLeaRCXString(httpHeader404)
		b.emitBytes(0x49, 0x89, 0xCF) // mov r15, rcx
		b.emitCallLabel("__joss_rt_strlen")
		b.emitBytes(0x49, 0x89, 0xC0)       // mov r8, rax
		b.emitBytes(0x4C, 0x89, 0xFA)       // mov rdx, r15
		b.emitBytes(0x4C, 0x89, 0xF1)       // mov rcx, r14
		b.emitBytes(0x45, 0x31, 0xC9)       // xor r9d, r9d
		b.emitBytes(0x48, 0x8B, 0x45, 0xC0) // mov rax, [rbp - 64] (pSend)
		b.emitBytes(0xFF, 0xD0)             // call rax
		b.emitJmpLabel(".srv_close_client")

		b.defineLabel(".srv_dispatch_route")
		if hasCompiledRouter {
			b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15 (pathPtr)
			b.emitCallLabel("fn___joss_http_dispatch")
			b.emitBytes(0x49, 0x89, 0xC7) // mov r15, rax (htmlBodyPtr)
		} else {
			b.emitLeaRCXString(defaultHTMLBody)
			b.emitBytes(0x49, 0x89, 0xCF) // mov r15, rcx
		}

		// Send HTTP 200 HTML header
		b.emitLeaRCXString(httpHeaderHTML)
		b.emitBytes(0x48, 0x89, 0xCB) // mov rbx, rcx
		b.emitCallLabel("__joss_rt_strlen")
		b.emitBytes(0x49, 0x89, 0xC0)       // mov r8, rax
		b.emitBytes(0x48, 0x89, 0xDA)       // mov rdx, rbx
		b.emitBytes(0x4C, 0x89, 0xF1)       // mov rcx, r14 (clientSock)
		b.emitBytes(0x45, 0x31, 0xC9)       // xor r9d, r9d
		b.emitBytes(0x48, 0x8B, 0x45, 0xC0) // mov rax, [rbp - 64] (pSend)
		b.emitBytes(0xFF, 0xD0)             // call rax

		// Send HTML body in r15
		b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15
		b.emitCallLabel("__joss_rt_strlen")
		b.emitBytes(0x49, 0x89, 0xC0)       // mov r8, rax (len)
		b.emitBytes(0x4C, 0x89, 0xFA)       // mov rdx, r15 (htmlBodyPtr)
		b.emitBytes(0x4C, 0x89, 0xF1)       // mov rcx, r14 (clientSock)
		b.emitBytes(0x45, 0x31, 0xC9)       // xor r9d, r9d
		b.emitBytes(0x48, 0x8B, 0x45, 0xC0) // mov rax, [rbp - 64] (pSend)
		b.emitBytes(0xFF, 0xD0)             // call rax

		// closesocket(clientSock)
		b.defineLabel(".srv_close_client")
		b.emitBytes(0x4C, 0x89, 0xF1)       // mov rcx, r14
		b.emitBytes(0x48, 0x8B, 0x45, 0xB8) // mov rax, [rbp - 72] (pClosesocket)
		b.emitBytes(0xFF, 0xD0)             // call rax
		b.emitJmpLabel(".srv_accept_loop")

		b.defineLabel(".srv_exit_ok")
		b.emitBytes(0x31, 0xC0)                                           // xor eax, eax
		b.emitBytes(0x48, 0x81, 0xC4, 0x98, 0x00, 0x00, 0x00)             // add rsp, 152
		b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5B) // pop r15..r12, rbx
		b.emitBytes(0x5D, 0xC3)
	} else {
		// Linux / Darwin x86_64 POSIX sockets HTTP server
		var (
			sysSocket     uint32 = 41
			sysSetsockopt uint32 = 54
			sysBind       uint32 = 49
			sysListen     uint32 = 50
			sysAccept     uint32 = 43
			sysRead       uint32 = 0
			sysWrite      uint32 = 1
			sysClose      uint32 = 3
			solSocket     uint32 = 1
			soReuseAddr   uint32 = 2
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
			solSocket = 0xFFFF
			soReuseAddr = 0x0004
		}

		b.emitBytes(0x55, 0x48, 0x89, 0xE5)                               // push rbp; mov rbp, rsp
		b.emitBytes(0x53, 0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57) // push rbx, r12..r15 (40 bytes)
		b.emitBytes(0x48, 0x81, 0xEC, 0x98, 0x00, 0x00, 0x00)             // sub rsp, 152

		b.emitCallLabel("__joss_load_env_file")
		b.emitLeaRCXString("🚀 [Joss Native Server] Escuchando en http://localhost:8000 (Ctrl+C para detener)")
		b.emitCallLabel("joss_print_string")

		// Allocate 16384-byte request buffer
		b.emitBytes(0xB9, 0x00, 0x40, 0x00, 0x00) // mov ecx, 16384
		b.emitCallLabel("__joss_os_alloc")
		b.emitBytes(0x48, 0x89, 0x45, 0xB0) // mov [rbp - 80], rax (reqBuf)

		// socket(AF_INET=2, SOCK_STREAM=1, 0)
		b.emitBytes(0xBF, 0x02, 0x00, 0x00, 0x00) // mov edi, 2
		b.emitBytes(0xBE, 0x01, 0x00, 0x00, 0x00) // mov esi, 1
		b.emitBytes(0x31, 0xD2)                   // xor edx, edx
		b.emitBytes(0xB8)
		b.emitU32(sysSocket)
		b.emitBytes(0x0F, 0x05)       // syscall
		b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
		b.emitJlLabel(".srv_posix_exit")
		b.emitBytes(0x49, 0x89, 0xC5) // mov r13, rax (listenSock)

		// setsockopt(listenSock, solSocket, soReuseAddr, &optval, 4)
		b.emitBytes(0xC7, 0x45, 0x98, 0x01, 0x00, 0x00, 0x00) // mov dword ptr [rbp - 104], 1
		b.emitBytes(0x4C, 0x89, 0xEF)                         // mov rdi, r13
		b.emitBytes(0xBE)                                     // mov esi, solSocket
		b.emitU32(solSocket)
		b.emitBytes(0xBA) // mov edx, soReuseAddr
		b.emitU32(soReuseAddr)
		b.emitBytes(0x4C, 0x8D, 0x55, 0x98)             // lea r10, [rbp - 104]
		b.emitBytes(0x41, 0xB8, 0x04, 0x00, 0x00, 0x00) // mov r8d, 4
		b.emitBytes(0xB8)
		b.emitU32(sysSetsockopt)
		b.emitBytes(0x0F, 0x05) // syscall

		// sockaddr_in at [rbp - 96]: port 8000 (0x1F40 -> 0x401F big-endian)
		if b.target.OS == "darwin" {
			b.emitBytes(0xC7, 0x45, 0xA0, 0x10, 0x02, 0x1F, 0x40) // sin_len=16, sin_family=2, sin_port=htons(8000)
		} else {
			b.emitBytes(0xC7, 0x45, 0xA0, 0x02, 0x00, 0x1F, 0x40) // sin_family=2, sin_port=htons(8000)
		}
		b.emitBytes(0xC7, 0x45, 0xA4, 0x00, 0x00, 0x00, 0x00)
		b.emitBytes(0x48, 0xC7, 0x45, 0xA8, 0x00, 0x00, 0x00, 0x00)

		// Lookup PORT in envMap
		b.emitMovRCXData(dataOffEnvMap)
		b.emitBytes(0x48, 0x85, 0xC9)
		b.emitJeLabel(".srv_posix_port_done")
		b.emitLeaRDXString("PORT")
		b.emitCallLabel("joss_map_get_i64")
		b.emitBytes(0x48, 0x85, 0xC0)
		b.emitJeLabel(".srv_posix_port_done")
		b.emitBytes(0x48, 0x89, 0xC1)
		b.emitBytes(0x31, 0xD2)
		b.defineLabel(".srv_posix_port_loop")
		b.emitBytes(0x0F, 0xB6, 0x01)
		b.emitBytes(0x83, 0xF8, 0x30)
		b.emitJlLabel(".srv_posix_port_parsed")
		b.emitBytes(0x83, 0xF8, 0x39)
		b.emitJgLabel(".srv_posix_port_parsed")
		b.emitBytes(0x83, 0xE8, 0x30)
		b.emitBytes(0x69, 0xD2, 0x0A, 0x00, 0x00, 0x00)
		b.emitBytes(0x01, 0xC2)
		b.emitBytes(0x48, 0xFF, 0xC1)
		b.emitJmpLabel(".srv_posix_port_loop")
		b.defineLabel(".srv_posix_port_parsed")
		b.emitBytes(0x85, 0xD2)
		b.emitJleLabel(".srv_posix_port_done")
		b.emitBytes(0x86, 0xD6)             // xchg dl, dh
		b.emitBytes(0x66, 0x89, 0x55, 0xA2) // mov word ptr [rbp - 94], dx
		b.defineLabel(".srv_posix_port_done")

		// bind(listenSock, &sockaddr_in, 16)
		b.emitBytes(0x4C, 0x89, 0xEF)             // mov rdi, r13
		b.emitBytes(0x48, 0x8D, 0x75, 0xA0)       // lea rsi, [rbp - 96]
		b.emitBytes(0xBA, 0x10, 0x00, 0x00, 0x00) // mov edx, 16
		b.emitBytes(0xB8)
		b.emitU32(sysBind)
		b.emitBytes(0x0F, 0x05) // syscall

		// listen(listenSock, 128)
		b.emitBytes(0x4C, 0x89, 0xEF)             // mov rdi, r13
		b.emitBytes(0xBE, 0x80, 0x00, 0x00, 0x00) // mov esi, 128
		b.emitBytes(0xB8)
		b.emitU32(sysListen)
		b.emitBytes(0x0F, 0x05) // syscall

		// Accept loop
		b.defineLabel(".srv_posix_accept_loop")
		b.emitBytes(0x4C, 0x89, 0xEF) // mov rdi, r13
		b.emitBytes(0x31, 0xF6)       // xor esi, esi
		b.emitBytes(0x31, 0xD2)       // xor edx, edx
		b.emitBytes(0xB8)
		b.emitU32(sysAccept)
		b.emitBytes(0x0F, 0x05)       // syscall
		b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
		b.emitJlLabel(".srv_posix_exit")
		b.emitBytes(0x49, 0x89, 0xC6) // mov r14, rax (clientSock)

		// read(clientSock, reqBuf, 16380)
		b.emitBytes(0x4C, 0x89, 0xF7)             // mov rdi, r14
		b.emitBytes(0x48, 0x8B, 0x75, 0xB0)       // mov rsi, [rbp - 80]
		b.emitBytes(0xBA, 0xFC, 0x3F, 0x00, 0x00) // mov edx, 16380
		b.emitBytes(0xB8)
		b.emitU32(sysRead)
		b.emitBytes(0x0F, 0x05)       // syscall
		b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
		b.emitJleLabel(".srv_posix_close_client")

		b.emitBytes(0x48, 0x8B, 0x55, 0xB0) // mov rdx, [rbp - 80]
		b.emitBytes(0xC6, 0x04, 0x02, 0x00) // mov byte ptr [rdx + rax], 0

		// Extract HTTP path into r15
		b.emitBytes(0x49, 0x89, 0xD7)
		b.defineLabel(".srv_posix_space1")
		b.emitBytes(0x41, 0x0F, 0xB6, 0x07)
		b.emitBytes(0x85, 0xC0)
		b.emitJeLabel(".srv_posix_def_path")
		b.emitBytes(0x83, 0xF8, 0x20)
		b.emitJeLabel(".srv_posix_found_sp1")
		b.emitBytes(0x49, 0xFF, 0xC7)
		b.emitJmpLabel(".srv_posix_space1")
		b.defineLabel(".srv_posix_found_sp1")
		b.emitBytes(0x49, 0xFF, 0xC7)
		b.emitBytes(0x4C, 0x89, 0xFB)
		b.defineLabel(".srv_posix_path_end")
		b.emitBytes(0x0F, 0xB6, 0x03)
		b.emitBytes(0x85, 0xC0)
		b.emitJeLabel(".srv_posix_term_path")
		b.emitBytes(0x83, 0xF8, 0x20)
		b.emitJeLabel(".srv_posix_term_path")
		b.emitBytes(0x83, 0xF8, 0x3F)
		b.emitJeLabel(".srv_posix_term_path")
		b.emitBytes(0x83, 0xF8, 0x0D)
		b.emitJeLabel(".srv_posix_term_path")
		b.emitBytes(0x83, 0xF8, 0x0A)
		b.emitJeLabel(".srv_posix_term_path")
		b.emitBytes(0x48, 0xFF, 0xC3)
		b.emitJmpLabel(".srv_posix_path_end")
		b.defineLabel(".srv_posix_term_path")
		b.emitBytes(0xC6, 0x03, 0x00)
		b.emitJmpLabel(".srv_posix_have_path")
		b.defineLabel(".srv_posix_def_path")
		b.emitLeaRCXString("/")
		b.emitBytes(0x49, 0x89, 0xCF)

		b.defineLabel(".srv_posix_have_path")
		if hasCompiledRouter {
			b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15
			b.emitCallLabel("fn___joss_http_dispatch")
			b.emitBytes(0x49, 0x89, 0xC7) // mov r15, rax
		} else {
			b.emitLeaRCXString(defaultHTMLBody)
			b.emitBytes(0x49, 0x89, 0xCF)
		}

		// write(clientSock, httpHeaderHTML, len)
		b.emitLeaRCXString(httpHeaderHTML)
		b.emitBytes(0x48, 0x89, 0xCB)
		b.emitCallLabel("__joss_rt_strlen")
		b.emitBytes(0x4C, 0x89, 0xF7) // mov rdi, r14
		b.emitBytes(0x48, 0x89, 0xDE) // mov rsi, rbx
		b.emitBytes(0x48, 0x89, 0xC2) // mov rdx, rax
		b.emitBytes(0xB8)
		b.emitU32(sysWrite)
		b.emitBytes(0x0F, 0x05)

		// write(clientSock, htmlBodyPtr, len)
		b.emitBytes(0x4C, 0x89, 0xF9)
		b.emitCallLabel("__joss_rt_strlen")
		b.emitBytes(0x4C, 0x89, 0xF7) // mov rdi, r14
		b.emitBytes(0x4C, 0x89, 0xFE) // mov rsi, r15
		b.emitBytes(0x48, 0x89, 0xC2) // mov rdx, rax
		b.emitBytes(0xB8)
		b.emitU32(sysWrite)
		b.emitBytes(0x0F, 0x05)

		b.defineLabel(".srv_posix_close_client")
		b.emitBytes(0x4C, 0x89, 0xF7) // mov rdi, r14
		b.emitBytes(0xB8)
		b.emitU32(sysClose)
		b.emitBytes(0x0F, 0x05)
		b.emitJmpLabel(".srv_posix_accept_loop")

		b.defineLabel(".srv_posix_exit")
		b.emitBytes(0x31, 0xC0)
		b.emitBytes(0x48, 0x81, 0xC4, 0x98, 0x00, 0x00, 0x00)
		b.emitBytes(0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5B)
		b.emitBytes(0x5D, 0xC3)
	}
}
