package native

// emitNativeServer emits the Socket networking primitives for x86_64 (Windows Winsock2 & POSIX syscalls)
// and delegates Server::start to Pure Joss Standard Library (fn_HttpServer_start).
func (b *MachineCodeBuilder) emitNativeServer() {
	if _, exists := b.labels["fn_Socket_listen"]; exists {
		return
	}

	// 1. Point of entry for Server::start -> jumps to Pure Joss HttpServer::start (if not defined in IR)
	if _, exists := b.labels["fn_Server_start"]; !exists {
		b.defineLabel("fn_Server_start")
		b.defineLabel("Server_start")
		b.defineLabel(".srv_accept_loop")
		b.emitJmpLabel("fn_HttpServer_start")
	}

	// 2. Windows Winsock initializer helper (now initialized in __joss_os_init)
	if b.target.OS == "windows" {
		b.defineLabel("__joss_ws2_init")
		b.emitBytes(0xC3) // ret
	}

	// 3. fn_Socket_listen(rcx/rdi = host, rdx/rsi = portStr) -> rax = Socket Object
	b.defineLabel("fn_Socket_listen")
	b.defineLabel("Socket_listen")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                   // push rbp; mov rbp, rsp
	b.emitBytes(0x53, 0x41, 0x54, 0x41, 0x55, 0x41, 0x56) // push rbx, r12..r14 (4 pushes = 32 bytes)
	b.emitBytes(0x48, 0x83, 0xEC, 0x40)                   // sub rsp, 64 (16-byte aligned before calls: 32+64=96)

	if b.target.OS == "windows" {
		// rcx = host, rdx = port (can be integer or string pointer)
		b.emitBytes(0x41, 0xBC, 0x50, 0x00, 0x00, 0x00) // mov r12d, 80 (default)
		b.emitBytes(0x48, 0x85, 0xD2)                   // test rdx, rdx
		b.emitJeLabel(".sock_win_have_port")
		b.emitBytes(0x48, 0x81, 0xFA, 0x00, 0x00, 0x01, 0x00) // cmp rdx, 0x10000
		b.emitJgeLabel(".sock_win_parse_str")
		// Integer port
		b.emitBytes(0x41, 0x89, 0xD4) // mov r12d, edx
		b.emitJmpLabel(".sock_win_have_port")

		b.defineLabel(".sock_win_parse_str")
		b.emitBytes(0x31, 0xDB)       // xor ebx, ebx
		b.defineLabel(".sock_win_parse_port")
		b.emitBytes(0x0F, 0xB6, 0x02) // movzx eax, byte ptr [rdx]
		b.emitBytes(0x84, 0xC0)       // test al, al
		b.emitJeLabel(".sock_win_parsed")
		b.emitBytes(0x3C, 0x30) // cmp al, '0'
		b.emitJbLabel(".sock_win_parsed")
		b.emitBytes(0x3C, 0x39) // cmp al, '9'
		b.emitJaLabel(".sock_win_parsed")
		b.emitBytes(0x2C, 0x30)       // sub al, '0'
		b.emitBytes(0x6B, 0xDB, 0x0A) // imul ebx, ebx, 10
		b.emitBytes(0x01, 0xC3)       // add ebx, eax
		b.emitBytes(0x48, 0xFF, 0xC2) // inc rdx
		b.emitJmpLabel(".sock_win_parse_port")
		b.defineLabel(".sock_win_parsed")
		b.emitBytes(0x85, 0xDB) // test ebx, ebx
		b.emitJeLabel(".sock_win_have_port")
		b.emitBytes(0x41, 0x89, 0xDC) // mov r12d, ebx
		b.defineLabel(".sock_win_have_port")

		// socket(AF_INET=2, SOCK_STREAM=1, IPPROTO_TCP=6)
		b.emitBytes(0xB9, 0x02, 0x00, 0x00, 0x00)       // mov ecx, 2
		b.emitBytes(0xBA, 0x01, 0x00, 0x00, 0x00)       // mov edx, 1
		b.emitBytes(0x41, 0xB8, 0x06, 0x00, 0x00, 0x00) // mov r8d, 6
		b.emitMovRAXData(dataOffSocket)
		b.emitBytes(0xFF, 0xD0)             // call socket
		b.emitBytes(0x48, 0x83, 0xF8, 0xFF) // cmp rax, -1
		b.emitJeLabel(".sock_listen_fail")
		b.emitBytes(0x49, 0x89, 0xC6) // mov r14, rax (listenSock)

		// setsockopt(listenSock, SOL_SOCKET=0xFFFF, SO_REUSEADDR=4, &optval=1, 4)
		b.emitBytes(0xC7, 0x44, 0x24, 0x28, 0x01, 0x00, 0x00, 0x00) // mov dword ptr [rsp + 40], 1 (optval)
		b.emitBytes(0x4C, 0x89, 0xF1)                               // mov rcx, r14 (listenSock)
		b.emitBytes(0xBA, 0xFF, 0xFF, 0x00, 0x00)                   // mov edx, 0xFFFF (SOL_SOCKET)
		b.emitBytes(0x41, 0xB8, 0x04, 0x00, 0x00, 0x00)             // mov r8d, 4 (SO_REUSEADDR)
		b.emitBytes(0x4C, 0x8D, 0x4C, 0x24, 0x28)                   // lea r9, [rsp + 40] (&optval)
		b.emitBytes(0xC7, 0x44, 0x24, 0x20, 0x04, 0x00, 0x00, 0x00) // mov dword ptr [rsp + 32], 4 (optlen)
		b.emitMovRAXData(dataOffSetsockopt)
		b.emitBytes(0xFF, 0xD0) // call setsockopt

		// sockaddr_in at [rsp + 48]
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x30, 0x00, 0x00, 0x00, 0x00) // zero sockaddr[0..7]
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x38, 0x00, 0x00, 0x00, 0x00) // zero sockaddr[8..15]
		b.emitBytes(0x66, 0xC7, 0x44, 0x24, 0x30, 0x02, 0x00)             // sin_family = AF_INET (2)
		// htons(r12d) -> sin_port at [rsp + 50]
		b.emitBytes(0x41, 0x8B, 0xC4)             // mov eax, r12d
		b.emitBytes(0x86, 0xC4)                   // xchg al, ah
		b.emitBytes(0x66, 0x89, 0x44, 0x24, 0x32) // mov [rsp + 50], ax

		// bind(listenSock, &sockaddr, 16)
		b.emitBytes(0x4C, 0x89, 0xF1)                    // mov rcx, r14
		b.emitBytes(0x48, 0x8D, 0x54, 0x24, 0x30)        // lea rdx, [rsp + 48]
		b.emitBytes(0x41, 0xB8, 0x10, 0x00, 0x00, 0x00) // mov r8d, 16
		b.emitMovRAXData(dataOffBind)
		b.emitBytes(0xFF, 0xD0) // call bind
		b.emitBytes(0x85, 0xC0) // test eax, eax
		b.emitJneLabel(".sock_listen_close_fail")

		// listen(listenSock, 128)
		b.emitBytes(0x4C, 0x89, 0xF1)             // mov rcx, r14
		b.emitBytes(0xBA, 0x80, 0x00, 0x00, 0x00) // mov edx, 128
		b.emitMovRAXData(dataOffListen)
		b.emitBytes(0xFF, 0xD0) // call listen
		b.emitBytes(0x85, 0xC0) // test eax, eax
		b.emitJneLabel(".sock_listen_close_fail")

		// Success: allocate Socket Object (16 bytes: [0]=className, [8]=fd)
		b.emitBytes(0xB9, 0x10, 0x00, 0x00, 0x00) // mov ecx, 16
		b.emitCallLabel("__joss_os_alloc")
		b.emitLeaRDXString("Socket")
		b.emitBytes(0x48, 0x89, 0x10)       // mov [rax], rdx
		b.emitBytes(0x4C, 0x89, 0x70, 0x08) // mov [rax + 8], r14 (sockFd)
		b.emitJmpLabel(".sock_listen_ret")

		b.defineLabel(".sock_listen_close_fail")
		b.emitBytes(0x4C, 0x89, 0xF1) // mov rcx, r14
		b.emitMovRAXData(dataOffClosesocket)
		b.emitBytes(0xFF, 0xD0) // call closesocket

		b.defineLabel(".sock_listen_fail")
		b.emitBytes(0x48, 0x31, 0xC0) // xor rax, rax (null)

		b.defineLabel(".sock_listen_ret")
		b.emitBytes(0x48, 0x83, 0xC4, 0x40) // add rsp, 64
		b.emitBytes(0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5B, 0x5D, 0xC3) // pop r14..r12, rbx, rbp; ret
	} else {
		// POSIX (Linux x86_64): rdi = host, rsi = port (int or string)
		b.emitBytes(0x41, 0xBC, 0x50, 0x00, 0x00, 0x00) // mov r12d, 80 (default)
		b.emitBytes(0x48, 0x85, 0xF6)                   // test rsi, rsi
		b.emitJeLabel(".sock_posix_have_port")
		b.emitBytes(0x48, 0x81, 0xFE, 0x00, 0x00, 0x01, 0x00) // cmp rsi, 0x10000
		b.emitJgeLabel(".sock_posix_parse_str")
		// Integer port
		b.emitBytes(0x41, 0x89, 0xF4) // mov r12d, esi
		b.emitJmpLabel(".sock_posix_have_port")

		b.defineLabel(".sock_posix_parse_str")
		b.emitBytes(0x48, 0x89, 0xF2) // mov rdx, rsi (cursor)
		b.emitBytes(0x31, 0xDB)       // xor ebx, ebx (port acc)
		b.defineLabel(".sock_posix_parse_port")
		b.emitBytes(0x0F, 0xB6, 0x02) // movzx eax, byte ptr [rdx]
		b.emitBytes(0x84, 0xC0)       // test al, al
		b.emitJeLabel(".sock_posix_parsed")
		b.emitBytes(0x3C, 0x30) // cmp al, '0'
		b.emitJbLabel(".sock_posix_parsed")
		b.emitBytes(0x3C, 0x39) // cmp al, '9'
		b.emitJaLabel(".sock_posix_parsed")
		b.emitBytes(0x2C, 0x30)       // sub al, '0'
		b.emitBytes(0x6B, 0xDB, 0x0A) // imul ebx, ebx, 10
		b.emitBytes(0x01, 0xC3)       // add ebx, eax
		b.emitBytes(0x48, 0xFF, 0xC2) // inc rdx
		b.emitJmpLabel(".sock_posix_parse_port")
		b.defineLabel(".sock_posix_parsed")
		b.emitBytes(0x85, 0xDB) // test ebx, ebx
		b.emitJeLabel(".sock_posix_have_port")
		b.emitBytes(0x41, 0x89, 0xDC) // mov r12d, ebx
		b.defineLabel(".sock_posix_have_port")

		// sysSocket = 41 (AF_INET=2, SOCK_STREAM=1, 0)
		b.emitBytes(0xBF, 0x02, 0x00, 0x00, 0x00) // mov edi, 2
		b.emitBytes(0xBE, 0x01, 0x00, 0x00, 0x00) // mov esi, 1
		b.emitBytes(0x31, 0xD2)                   // xor edx, edx
		b.emitBytes(0xB8, 0x29, 0x00, 0x00, 0x00) // mov eax, 41
		b.emitBytes(0x0F, 0x05)                   // syscall
		b.emitBytes(0x48, 0x85, 0xC0)             // test rax, rax
		b.emitJlLabel(".sock_posix_fail")
		b.emitBytes(0x49, 0x89, 0xC6) // mov r14, rax (fd)

		// setsockopt(fd, SOL_SOCKET=1, SO_REUSEADDR=2, &opt=1, 4)
		b.emitBytes(0xC7, 0x45, 0x90, 0x01, 0x00, 0x00, 0x00) // mov dword ptr [rbp-112], 1
		b.emitBytes(0x4C, 0x89, 0xF7)                         // mov rdi, r14
		b.emitBytes(0xBE, 0x01, 0x00, 0x00, 0x00)             // mov esi, 1 (SOL_SOCKET)
		b.emitBytes(0xBA, 0x02, 0x00, 0x00, 0x00)             // mov edx, 2 (SO_REUSEADDR)
		b.emitBytes(0x48, 0x8D, 0x4D, 0x90)                   // lea rcx, [rbp-112]
		b.emitBytes(0x49, 0x89, 0xCA)                         // mov r10, rcx
		b.emitBytes(0x41, 0xB8, 0x04, 0x00, 0x00, 0x00)       // mov r8d, 4
		b.emitBytes(0xB8, 0x36, 0x00, 0x00, 0x00)             // mov eax, 54 (sysSetsockopt)
		b.emitBytes(0x0F, 0x05)

		// sockaddr_in at [rbp - 96]
		b.emitBytes(0x48, 0xC7, 0x45, 0xA0, 0x00, 0x00, 0x00, 0x00)
		b.emitBytes(0x48, 0xC7, 0x45, 0xA8, 0x00, 0x00, 0x00, 0x00)
		b.emitBytes(0x66, 0xC7, 0x45, 0xA0, 0x02, 0x00) // sin_family = 2
		b.emitBytes(0x41, 0x8B, 0xC4)                   // mov eax, r12d
		b.emitBytes(0x86, 0xC4)                         // xchg al, ah
		b.emitBytes(0x66, 0x89, 0x45, 0xA2)             // sin_port

		// sysBind = 49 (fd, &sockaddr, 16)
		b.emitBytes(0x4C, 0x89, 0xF7)             // mov rdi, r14
		b.emitBytes(0x48, 0x8D, 0x75, 0xA0)       // lea rsi, [rbp - 96]
		b.emitBytes(0xBA, 0x10, 0x00, 0x00, 0x00) // mov edx, 16
		b.emitBytes(0xB8, 0x31, 0x00, 0x00, 0x00) // mov eax, 49
		b.emitBytes(0x0F, 0x05)                   // syscall
		b.emitBytes(0x48, 0x85, 0xC0)
		b.emitJneLabel(".sock_posix_close_fail")

		// sysListen = 50 (fd, 128)
		b.emitBytes(0x4C, 0x89, 0xF7)             // mov rdi, r14
		b.emitBytes(0xBE, 0x80, 0x00, 0x00, 0x00) // mov esi, 128
		b.emitBytes(0xB8, 0x32, 0x00, 0x00, 0x00) // mov eax, 50
		b.emitBytes(0x0F, 0x05)
		b.emitBytes(0x48, 0x85, 0xC0)
		b.emitJneLabel(".sock_posix_close_fail")

		// Allocate Socket Object
		b.emitBytes(0xBF, 0x10, 0x00, 0x00, 0x00) // mov edi, 16
		b.emitCallLabel("__joss_os_alloc")
		b.emitLeaRDXString("Socket")
		b.emitBytes(0x48, 0x89, 0x10)       // [rax] = "Socket"
		b.emitBytes(0x4C, 0x89, 0x70, 0x08) // [rax+8] = fd
		b.emitJmpLabel(".sock_posix_ret")

		b.defineLabel(".sock_posix_close_fail")
		b.emitBytes(0x4C, 0x89, 0xF7)             // mov rdi, r14
		b.emitBytes(0xB8, 0x03, 0x00, 0x00, 0x00) // mov eax, 3 (sysClose)
		b.emitBytes(0x0F, 0x05)

		b.defineLabel(".sock_posix_fail")
		b.emitBytes(0x48, 0x31, 0xC0)

		b.defineLabel(".sock_posix_ret")
		b.emitBytes(0x48, 0x81, 0xC4, 0x80, 0x00, 0x00, 0x00)
		b.emitBytes(0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5B, 0x5D, 0xC3) // pop r14..r12, rbx, rbp; ret
	}

	// 4. fn_Socket_accept(sockObj) -> rax = client Socket Object (or 0)
	b.defineLabel("fn_Socket_accept")
	b.defineLabel("Socket_accept")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                   // push rbp; mov rbp, rsp
	b.emitBytes(0x53, 0x41, 0x54, 0x41, 0x55, 0x41, 0x56) // push rbx, r12..r14
	b.emitBytes(0x48, 0x83, 0xEC, 0x30)                   // sub rsp, 48

	if b.target.OS == "windows" {
		b.emitBytes(0x48, 0x85, 0xC9) // test rcx, rcx
		b.emitJeLabel(".sock_accept_fail")
		b.emitBytes(0x48, 0x8B, 0x49, 0x08) // mov rcx, [rcx + 8] (listenSock)
		b.emitBytes(0x48, 0x85, 0xC9)       // test rcx, rcx
		b.emitJeLabel(".sock_accept_fail")
		b.emitBytes(0x31, 0xD2)             // xor edx, edx
		b.emitBytes(0x45, 0x31, 0xC0)       // xor r8d, r8d
		b.emitMovRAXData(dataOffAccept)
		b.emitBytes(0xFF, 0xD0)             // call accept
		b.emitBytes(0x48, 0x83, 0xF8, 0xFF) // cmp rax, -1
		b.emitJeLabel(".sock_accept_fail")
		b.emitBytes(0x49, 0x89, 0xC6) // mov r14, rax (clientSock)

		// Allocate client Socket Object
		b.emitBytes(0xB9, 0x10, 0x00, 0x00, 0x00) // mov ecx, 16
		b.emitCallLabel("__joss_os_alloc")
		b.emitLeaRDXString("Socket")
		b.emitBytes(0x48, 0x89, 0x10)       // [rax] = "Socket"
		b.emitBytes(0x4C, 0x89, 0x70, 0x08) // [rax+8] = clientSock
		b.emitJmpLabel(".sock_accept_ret")

		b.defineLabel(".sock_accept_fail")
		b.emitBytes(0x48, 0x31, 0xC0)

		b.defineLabel(".sock_accept_ret")
		b.emitBytes(0x48, 0x83, 0xC4, 0x30)
		b.emitBytes(0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5B, 0x5D, 0xC3)
	} else {
		// POSIX
		b.emitBytes(0x48, 0x85, 0xFF) // test rdi, rdi
		b.emitJeLabel(".sock_posix_acc_fail")
		b.emitBytes(0x48, 0x8B, 0x7F, 0x08) // mov rdi, [rdi + 8]
		b.emitBytes(0x48, 0x85, 0xFF)
		b.emitJeLabel(".sock_posix_acc_fail")
		b.emitBytes(0x31, 0xF6)                   // xor esi, esi
		b.emitBytes(0x31, 0xD2)                   // xor edx, edx
		b.emitBytes(0xB8, 0x2B, 0x00, 0x00, 0x00) // mov eax, 43 (sysAccept)
		b.emitBytes(0x0F, 0x05)
		b.emitBytes(0x48, 0x85, 0xC0)
		b.emitJlLabel(".sock_posix_acc_fail")
		b.emitBytes(0x49, 0x89, 0xC6) // mov r14, rax

		b.emitBytes(0xBF, 0x10, 0x00, 0x00, 0x00) // mov edi, 16
		b.emitCallLabel("__joss_os_alloc")
		b.emitLeaRDXString("Socket")
		b.emitBytes(0x48, 0x89, 0x10)
		b.emitBytes(0x4C, 0x89, 0x70, 0x08)
		b.emitJmpLabel(".sock_posix_acc_ret")

		b.defineLabel(".sock_posix_acc_fail")
		b.emitBytes(0x48, 0x31, 0xC0)

		b.defineLabel(".sock_posix_acc_ret")
		b.emitBytes(0x48, 0x83, 0xC4, 0x30)
		b.emitBytes(0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5B, 0x5D, 0xC3)
	}

	// 5. fn_Socket_receive(clientObj, maxBytes) -> rax = string buffer
	b.defineLabel("fn_Socket_receive")
	b.defineLabel("Socket_receive")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                   // push rbp; mov rbp, rsp
	b.emitBytes(0x53, 0x41, 0x54, 0x41, 0x55, 0x41, 0x56) // push rbx, r12..r14
	b.emitBytes(0x48, 0x83, 0xEC, 0x30)                   // sub rsp, 48

	if b.target.OS == "windows" {
		b.emitBytes(0x48, 0x85, 0xC9)
		b.emitJeLabel(".sock_recv_empty")
		b.emitBytes(0x4C, 0x8B, 0x71, 0x08) // mov r14, [rcx + 8] (clientSock)
		b.emitBytes(0x4D, 0x85, 0xF6)
		b.emitJeLabel(".sock_recv_empty")

		// Clamp maxBytes to 4096 if 0
		b.emitBytes(0x48, 0x85, 0xD2)
		b.emitJgLabel(".sock_win_have_max")
		b.emitBytes(0xBA, 0x00, 0x10, 0x00, 0x00) // mov edx, 4096
		b.defineLabel(".sock_win_have_max")
		b.emitBytes(0x41, 0x89, 0xD4) // mov r12d, edx (maxBytes)

		// Allocate buffer (maxBytes + 1)
		b.emitBytes(0x8D, 0x4A, 0x01) // lea ecx, [rdx + 1]
		b.emitCallLabel("__joss_os_alloc")
		b.emitBytes(0x48, 0x89, 0xC3) // mov rbx, rax (bufPtr)

		// recv(clientSock, bufPtr, maxBytes, 0)
		b.emitBytes(0x4C, 0x89, 0xF1) // mov rcx, r14 (sock)
		b.emitBytes(0x48, 0x89, 0xDA) // mov rdx, rbx (buf)
		b.emitBytes(0x45, 0x89, 0xE0) // mov r8d, r12d (len)
		b.emitBytes(0x45, 0x31, 0xC9) // mov r9d, 0 (flags)
		b.emitMovRAXData(dataOffRecv)
		b.emitBytes(0xFF, 0xD0) // call recv

		b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
		b.emitJleLabel(".sock_recv_nullterm_zero")
		// bufPtr[rax] = 0
		b.emitBytes(0xC6, 0x04, 0x03, 0x00) // mov byte ptr [rbx + rax], 0
		b.emitBytes(0x48, 0x89, 0xD8)       // mov rax, rbx
		b.emitJmpLabel(".sock_recv_ret")

		b.defineLabel(".sock_recv_nullterm_zero")
		b.emitBytes(0xC6, 0x03, 0x00) // mov byte ptr [rbx], 0
		b.emitBytes(0x48, 0x89, 0xD8) // mov rax, rbx
		b.emitJmpLabel(".sock_recv_ret")

		b.defineLabel(".sock_recv_empty")
		b.emitLeaRCXString("")
		b.emitBytes(0x48, 0x89, 0xC8)

		b.defineLabel(".sock_recv_ret")
		b.emitBytes(0x48, 0x83, 0xC4, 0x30)
		b.emitBytes(0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5B, 0x5D, 0xC3)
	} else {
		// POSIX
		b.emitBytes(0x48, 0x85, 0xFF) // test rdi, rdi
		b.emitJeLabel(".sock_posix_recv_empty")
		b.emitBytes(0x4C, 0x8B, 0x77, 0x08) // mov r14, [rdi + 8]
		b.emitBytes(0x4D, 0x85, 0xF6)
		b.emitJeLabel(".sock_posix_recv_empty")

		b.emitBytes(0x48, 0x85, 0xF6) // test rsi, rsi
		b.emitJgLabel(".sock_posix_have_max")
		b.emitBytes(0xBE, 0x00, 0x10, 0x00, 0x00) // mov esi, 4096
		b.defineLabel(".sock_posix_have_max")
		b.emitBytes(0x41, 0x89, 0xF4) // mov r12d, esi

		// Allocate (maxBytes + 1)
		b.emitBytes(0x8D, 0x7E, 0x01) // lea edi, [rsi + 1]
		b.emitCallLabel("__joss_os_alloc")
		b.emitBytes(0x49, 0x89, 0xC5) // mov r13, rax

		// sysRead = 0 (clientSock, bufPtr, maxBytes)
		b.emitBytes(0x4C, 0x89, 0xF7) // mov rdi, r14
		b.emitBytes(0x4C, 0x89, 0xEE) // mov rsi, r13
		b.emitBytes(0x44, 0x89, 0xE2) // mov edx, r12d
		b.emitBytes(0x31, 0xC0)       // mov eax, 0
		b.emitBytes(0x0F, 0x05)

		b.emitBytes(0x48, 0x85, 0xC0)
		b.emitJleLabel(".sock_posix_recv_zero")
		b.emitBytes(0x42, 0xC6, 0x04, 0x05, 0x00) // [r13 + rax] = 0
		b.emitBytes(0x4C, 0x89, 0xE8)
		b.emitJmpLabel(".sock_posix_recv_ret")

		b.defineLabel(".sock_posix_recv_zero")
		b.emitBytes(0x41, 0xC6, 0x05, 0x00)
		b.emitBytes(0x4C, 0x89, 0xE8)
		b.emitJmpLabel(".sock_posix_recv_ret")

		b.defineLabel(".sock_posix_recv_empty")
		b.emitLeaRCXString("")
		b.emitBytes(0x48, 0x89, 0xC8)

		b.defineLabel(".sock_posix_recv_ret")
		b.emitBytes(0x48, 0x83, 0xC4, 0x30)
		b.emitBytes(0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5B, 0x5D, 0xC3)
	}

	// 6. fn_Socket_send(clientObj, strPtr) -> rax = bytesSent
	b.defineLabel("fn_Socket_send")
	b.defineLabel("Socket_send")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5)                   // push rbp; mov rbp, rsp
	b.emitBytes(0x53, 0x41, 0x54, 0x41, 0x55, 0x41, 0x56) // push rbx, r12..r14
	b.emitBytes(0x48, 0x83, 0xEC, 0x30)                   // sub rsp, 48

	if b.target.OS == "windows" {
		b.emitBytes(0x48, 0x85, 0xC9)
		b.emitJeLabel(".sock_send_zero")
		b.emitBytes(0x4C, 0x8B, 0x71, 0x08) // mov r14, [rcx + 8] (clientSock)
		b.emitBytes(0x4D, 0x85, 0xF6)
		b.emitJeLabel(".sock_send_zero")
		b.emitBytes(0x49, 0x89, 0xD5) // mov r13, rdx (save strPtr)

		// Calculate strlen(strPtr)
		b.emitBytes(0x48, 0x89, 0xD1) // mov rcx, rdx
		b.emitCallLabel("__joss_rt_strlen")
		b.emitBytes(0x49, 0x89, 0xC4) // mov r12, rax (len)
		b.emitBytes(0x48, 0x85, 0xC0)
		b.emitJeLabel(".sock_send_zero")

		// send(clientSock, strPtr, len, 0)
		b.emitBytes(0x4C, 0x89, 0xF1) // mov rcx, r14 (sock)
		b.emitBytes(0x4C, 0x89, 0xEA) // mov rdx, r13 (strPtr)
		b.emitBytes(0x4D, 0x89, 0xE0) // mov r8, r12 (len)
		b.emitBytes(0x45, 0x31, 0xC9) // mov r9d, 0 (flags)
		b.emitMovRAXData(dataOffSend)
		b.emitBytes(0xFF, 0xD0) // call send
		b.emitJmpLabel(".sock_send_ret")

		b.defineLabel(".sock_send_zero")
		b.emitBytes(0x48, 0x31, 0xC0)

		b.defineLabel(".sock_send_ret")
		b.emitBytes(0x48, 0x83, 0xC4, 0x30)
		b.emitBytes(0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5B, 0x5D, 0xC3)
	} else {
		// POSIX
		b.emitBytes(0x48, 0x85, 0xFF)
		b.emitJeLabel(".sock_posix_send_zero")
		b.emitBytes(0x4C, 0x8B, 0x77, 0x08) // mov r14, [rdi + 8]
		b.emitBytes(0x4D, 0x85, 0xF6)
		b.emitJeLabel(".sock_posix_send_zero")
		b.emitBytes(0x49, 0x89, 0xF5) // mov r13, rsi (strPtr)

		// strlen
		b.emitBytes(0x48, 0x89, 0xF7) // mov rdi, rsi
		b.emitCallLabel("__joss_rt_strlen")
		b.emitBytes(0x49, 0x89, 0xC4) // mov r12, rax
		b.emitBytes(0x48, 0x85, 0xC0)
		b.emitJeLabel(".sock_posix_send_zero")

		// sysWrite = 1 (clientSock, strPtr, len)
		b.emitBytes(0x4C, 0x89, 0xF7)             // mov rdi, r14
		b.emitBytes(0x4C, 0x89, 0xEE)             // mov rsi, r13
		b.emitBytes(0x4C, 0x89, 0xE2)             // mov rdx, r12
		b.emitBytes(0xB8, 0x01, 0x00, 0x00, 0x00) // mov eax, 1
		b.emitBytes(0x0F, 0x05)
		b.emitJmpLabel(".sock_posix_send_ret")

		b.defineLabel(".sock_posix_send_zero")
		b.emitBytes(0x48, 0x31, 0xC0)

		b.defineLabel(".sock_posix_send_ret")
		b.emitBytes(0x48, 0x83, 0xC4, 0x30)
		b.emitBytes(0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5B, 0x5D, 0xC3)
	}

	// 7. fn_Socket_close(sockObj) -> void
	b.defineLabel("fn_Socket_close")
	b.defineLabel("Socket_close")
	b.emitBytes(0x55, 0x48, 0x89, 0xE5) // push rbp; mov rbp, rsp
	b.emitBytes(0x48, 0x83, 0xEC, 0x20) // sub rsp, 32

	if b.target.OS == "windows" {
		b.emitBytes(0x48, 0x85, 0xC9)
		b.emitJeLabel(".sock_close_done")
		b.emitBytes(0x48, 0x8B, 0x49, 0x08) // mov rcx, [rcx + 8] (fd)
		b.emitBytes(0x48, 0x85, 0xC9)
		b.emitJeLabel(".sock_close_done")
		b.emitMovRAXData(dataOffClosesocket)
		b.emitBytes(0xFF, 0xD0) // call closesocket
		b.defineLabel(".sock_close_done")
		b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3)
	} else {
		b.emitBytes(0x48, 0x85, 0xFF)
		b.emitJeLabel(".sock_posix_close_done")
		b.emitBytes(0x48, 0x8B, 0x7F, 0x08) // mov rdi, [rdi + 8]
		b.emitBytes(0x48, 0x85, 0xFF)
		b.emitJeLabel(".sock_posix_close_done")
		b.emitBytes(0xB8, 0x03, 0x00, 0x00, 0x00) // mov eax, 3 (sysClose)
		b.emitBytes(0x0F, 0x05)
		b.defineLabel(".sock_posix_close_done")
		b.emitBytes(0x48, 0x89, 0xEC, 0x5D, 0xC3)
	}
}
