package native

// emitNativeServer emits the Socket networking primitives for AArch64 (Linux POSIX & Darwin syscalls)
// and delegates Server::start to Pure Joss Standard Library (fn_HttpServer_start).
func (b *ARM64Builder) emitNativeServer() {
	if _, exists := b.labels["fn_Socket_listen"]; exists {
		return
	}

	// 1. Point of entry for Server::start -> jumps to Pure Joss HttpServer::start (if not defined in IR)
	if _, exists := b.labels["fn_Server_start"]; !exists {
		b.defineLabel("fn_Server_start")
		b.defineLabel("Server_start")
		b.defineLabel(".srv_accept_loop")
		b.emitBranch("fn_HttpServer_start")
	}

	var (
		sysSocket     uint64 = 198
		sysSetsockopt uint64 = 208
		sysBind       uint64 = 200
		sysListen     uint64 = 201
		sysAccept     uint64 = 242 // __NR_accept4 on Linux AArch64
		sysRead       uint64 = 63
		sysWrite      uint64 = 64
		sysClose      uint64 = 57
	)
	if b.target.OS == "darwin" {
		sysSocket = 0x2000061
		sysSetsockopt = 0x2000069
		sysBind = 0x2000068
		sysListen = 0x200006A
		sysAccept = 0x200001E // SYS_accept on Darwin
		sysRead = 0x2000003
		sysWrite = 0x2000004
		sysClose = 0x2000006
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

	// 2. fn_Socket_listen(x0 = host, x1 = portStr) -> x0 = Socket Object
	b.defineLabel("fn_Socket_listen")
	b.defineLabel("Socket_listen")
	b.emitProlog(128)

	// Save port into x19
	b.emitMovReg(regX19, regX1)

	// Default port = 80 in x22
	b.emitLoadImm64(regX22, 80)
	b.emitCmp(regX19, regXZR)
	b.emitBranchCond(condEQ, ".arm_sock_have_port")
	b.emitLoadImm64(regX3, 0x10000)
	b.emitCmp(regX19, regX3)
	b.emitBranchCond(condCS, ".arm_sock_parse_str")
	// Integer port
	b.emitMovReg(regX22, regX19)
	b.emitBranch(".arm_sock_have_port")

	b.defineLabel(".arm_sock_parse_str")
	// Parse ASCII integer from x19 into x22
	b.emitLoadImm64(regX21, 0) // acc = 0
	b.defineLabel(".arm_sock_parse_port")
	b.emitLdrb(regX2, regX19, 0)
	b.emitCmp(regX2, regXZR)
	b.emitBranchCond(condEQ, ".arm_sock_parsed")
	b.emitSubImm(regX2, regX2, 48) // sub '0'
	b.emitLoadImm64(regX3, 10)
	b.emitMul(regX21, regX21, regX3)
	b.emitAdd(regX21, regX21, regX2)
	b.emitAddImm(regX19, regX19, 1)
	b.emitBranch(".arm_sock_parse_port")
	b.defineLabel(".arm_sock_parsed")
	b.emitCmp(regX21, regXZR)
	b.emitBranchCond(condEQ, ".arm_sock_have_port")
	b.emitMovReg(regX22, regX21)
	b.defineLabel(".arm_sock_have_port")

	// socket(AF_INET=2, SOCK_STREAM=1, 0)
	b.emitLoadImm64(regX0, 2)
	b.emitLoadImm64(regX1, 1)
	b.emitLoadImm64(regX2, 0)
	emitSyscallNum(sysSocket)
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condLT, ".arm_sock_fail")
	b.emitMovReg(regX19, regX0) // x19 = sockFd

	// setsockopt(sockFd, SOL_SOCKET=1, SO_REUSEADDR=2, &optval=1, 4)
	b.emitLoadImm64(regX0, 1)
	b.emitStr(regX0, regX29, 80) // [x29, #80] = 1
	b.emitMovReg(regX0, regX19)
	b.emitLoadImm64(regX1, 1)
	b.emitLoadImm64(regX2, 2)
	b.emitAddImm(regX3, regX29, 80)
	b.emitLoadImm64(regX4, 4)
	emitSyscallNum(sysSetsockopt)

	// Prepare sockaddr_in at [x29, #64]
	b.emitStr(regXZR, regX29, 64)
	b.emitStr(regXZR, regX29, 72)
	b.emitLoadImm64(regX0, 2) // AF_INET
	b.emitStrb(regX0, regX29, 64)

	// htons(x22)
	b.emitLoadImm64(regX3, 256)
	b.emitSDiv(regX4, regX22, regX3) // high byte
	b.emitMul(regX5, regX4, regX3)
	b.emitSub(regX6, regX22, regX5)  // low byte
	b.emitStrb(regX4, regX29, 66)    // high byte (network order)
	b.emitStrb(regX6, regX29, 67)    // low byte (network order)

	// bind(sockFd, &sockaddr, 16)
	b.emitMovReg(regX0, regX19)
	b.emitAddImm(regX1, regX29, 64)
	b.emitLoadImm64(regX2, 16)
	emitSyscallNum(sysBind)
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condNE, ".arm_sock_close_fail")

	// listen(sockFd, 128)
	b.emitMovReg(regX0, regX19)
	b.emitLoadImm64(regX1, 128)
	emitSyscallNum(sysListen)
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condNE, ".arm_sock_close_fail")

	// Allocate Socket Object (16 bytes: [0]=className, [8]=fd)
	b.emitLoadImm64(regX0, 16)
	b.emitCall("__joss_os_alloc")
	b.emitMovReg(regX20, regX0) // x20 = obj
	b.emitLoadStringAddr(regX1, "Socket")
	b.emitStr(regX1, regX20, 0)
	b.emitStr(regX19, regX20, 8)
	b.emitMovReg(regX0, regX20)
	b.emitBranch(".arm_sock_ret")

	b.defineLabel(".arm_sock_close_fail")
	b.emitMovReg(regX0, regX19)
	emitSyscallNum(sysClose)

	b.defineLabel(".arm_sock_fail")
	b.emitLoadImm64(regX0, 0)

	b.defineLabel(".arm_sock_ret")
	b.emitEpilog(128)

	// 3. fn_Socket_accept(sockObj) -> x0 = clientObj
	b.defineLabel("fn_Socket_accept")
	b.defineLabel("Socket_accept")
	b.emitProlog(64)

	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condEQ, ".arm_acc_fail")
	b.emitLdr(regX19, regX0, 8) // x19 = listenSock
	b.emitCmp(regX19, regXZR)
	b.emitBranchCond(condEQ, ".arm_acc_fail")

	// accept(listenSock, 0, 0)
	b.emitMovReg(regX0, regX19)
	b.emitLoadImm64(regX1, 0)
	b.emitLoadImm64(regX2, 0)
	b.emitLoadImm64(regX3, 0)
	emitSyscallNum(sysAccept)
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condLT, ".arm_acc_fail")
	b.emitMovReg(regX19, regX0) // clientFd

	// Allocate client object
	b.emitLoadImm64(regX0, 16)
	b.emitCall("__joss_os_alloc")
	b.emitMovReg(regX20, regX0)
	b.emitLoadStringAddr(regX1, "Socket")
	b.emitStr(regX1, regX20, 0)
	b.emitStr(regX19, regX20, 8)
	b.emitMovReg(regX0, regX20)
	b.emitBranch(".arm_acc_ret")

	b.defineLabel(".arm_acc_fail")
	b.emitLoadImm64(regX0, 0)

	b.defineLabel(".arm_acc_ret")
	b.emitEpilog(64)

	// 4. fn_Socket_receive(clientObj, maxBytes) -> x0 = string
	b.defineLabel("fn_Socket_receive")
	b.defineLabel("Socket_receive")
	b.emitProlog(64)

	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condEQ, ".arm_recv_empty")
	b.emitLdr(regX19, regX0, 8) // clientFd
	b.emitCmp(regX19, regXZR)
	b.emitBranchCond(condEQ, ".arm_recv_empty")

	b.emitCmp(regX1, regXZR)
	b.emitBranchCond(condGT, ".arm_recv_have_max")
	b.emitLoadImm64(regX1, 4096)
	b.defineLabel(".arm_recv_have_max")
	b.emitMovReg(regX20, regX1) // maxBytes

	// Allocate buffer (maxBytes + 1)
	b.emitAddImm(regX0, regX20, 1)
	b.emitCall("__joss_os_alloc")
	b.emitMovReg(regX21, regX0) // bufPtr

	// sysRead = 63 (clientFd, bufPtr, maxBytes)
	b.emitMovReg(regX0, regX19)
	b.emitMovReg(regX1, regX21)
	b.emitMovReg(regX2, regX20)
	emitSyscallNum(sysRead)
	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condLE, ".arm_recv_zero")
	b.emitAdd(regX2, regX21, regX0)
	b.emitStrb(regXZR, regX2, 0) // buf[n] = 0
	b.emitMovReg(regX0, regX21)
	b.emitBranch(".arm_recv_ret")

	b.defineLabel(".arm_recv_zero")
	b.emitStrb(regXZR, regX21, 0)
	b.emitMovReg(regX0, regX21)
	b.emitBranch(".arm_recv_ret")

	b.defineLabel(".arm_recv_empty")
	b.emitLoadStringAddr(regX0, "")

	b.defineLabel(".arm_recv_ret")
	b.emitEpilog(64)

	// 5. fn_Socket_send(clientObj, strPtr) -> x0 = bytesSent
	b.defineLabel("fn_Socket_send")
	b.defineLabel("Socket_send")
	b.emitProlog(64)

	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condEQ, ".arm_send_zero")
	b.emitLdr(regX19, regX0, 8) // clientFd
	b.emitCmp(regX19, regXZR)
	b.emitBranchCond(condEQ, ".arm_send_zero")
	b.emitMovReg(regX20, regX1) // strPtr

	b.emitMovReg(regX0, regX20)
	b.emitCall("__joss_rt_strlen")
	b.emitMovReg(regX21, regX0) // len
	b.emitCmp(regX21, regXZR)
	b.emitBranchCond(condEQ, ".arm_send_zero")

	// sysWrite = 64 (clientFd, strPtr, len)
	b.emitMovReg(regX0, regX19)
	b.emitMovReg(regX1, regX20)
	b.emitMovReg(regX2, regX21)
	emitSyscallNum(sysWrite)
	b.emitBranch(".arm_send_ret")

	b.defineLabel(".arm_send_zero")
	b.emitLoadImm64(regX0, 0)

	b.defineLabel(".arm_send_ret")
	b.emitEpilog(64)

	// 6. fn_Socket_close(sockObj) -> void
	b.defineLabel("fn_Socket_close")
	b.defineLabel("Socket_close")
	b.emitProlog(32)

	b.emitCmp(regX0, regXZR)
	b.emitBranchCond(condEQ, ".arm_close_done")
	b.emitLdr(regX19, regX0, 8) // fd
	b.emitCmp(regX19, regXZR)
	b.emitBranchCond(condEQ, ".arm_close_done")

	b.emitMovReg(regX0, regX19)
	emitSyscallNum(sysClose)

	b.defineLabel(".arm_close_done")
	b.emitEpilog(32)
}
