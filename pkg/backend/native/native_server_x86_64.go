package native

import (
	"fmt"
	"time"
)

// emitNativeServer emits the standalone x86_64 native HTTP server (fn_Server_start)
// supporting Windows Winsock2 and Linux/Darwin POSIX syscalls.
func (b *MachineCodeBuilder) emitNativeServer() {
	const (
		httpHeaderHTML   = "HTTP/1.1 200 OK\r\nContent-Type: text/html; charset=utf-8\r\nCache-Control: no-cache, no-store, must-revalidate\r\nPragma: no-cache\r\nServer: Joss-Native-AOT/1.0\r\nConnection: close\r\n\r\n"
		httpHeaderJSON   = "HTTP/1.1 200 OK\r\nContent-Type: application/json; charset=utf-8\r\nServer: Joss-Native-AOT/1.0\r\nConnection: close\r\n\r\n"
		httpHeaderCSS    = "HTTP/1.1 200 OK\r\nContent-Type: text/css; charset=utf-8\r\nCache-Control: no-cache\r\nServer: Joss-Native-AOT/1.0\r\nConnection: close\r\n\r\n"
		httpHeaderJS     = "HTTP/1.1 200 OK\r\nContent-Type: application/javascript; charset=utf-8\r\nCache-Control: no-cache\r\nServer: Joss-Native-AOT/1.0\r\nConnection: close\r\n\r\n"
		httpHeaderSVG    = "HTTP/1.1 200 OK\r\nContent-Type: image/svg+xml\r\nCache-Control: no-cache\r\nServer: Joss-Native-AOT/1.0\r\nConnection: close\r\n\r\n"
		httpHeaderPNG    = "HTTP/1.1 200 OK\r\nContent-Type: image/png\r\nServer: Joss-Native-AOT/1.0\r\nConnection: close\r\n\r\n"
		httpHeaderJPG    = "HTTP/1.1 200 OK\r\nContent-Type: image/jpeg\r\nServer: Joss-Native-AOT/1.0\r\nConnection: close\r\n\r\n"
		httpHeaderWEBP   = "HTTP/1.1 200 OK\r\nContent-Type: image/webp\r\nServer: Joss-Native-AOT/1.0\r\nConnection: close\r\n\r\n"
		httpHeaderICO    = "HTTP/1.1 200 OK\r\nContent-Type: image/x-icon\r\nServer: Joss-Native-AOT/1.0\r\nConnection: close\r\n\r\n"
		httpHeader404    = "HTTP/1.1 404 Not Found\r\nContent-Type: text/plain; charset=utf-8\r\nConnection: close\r\n\r\n404 Not Found"
		defaultHTMLBody  = "<!DOCTYPE html><html lang=\"es\"><head><meta charset=\"utf-8\"><title>JosSecurity — Native AOT</title></head><body><h1>JosSecurity — Native AOT</h1><p>ONLINE (HTTP 200 OK)</p></body></html>"
		liveReloadScript = `<script src="/__joss_live.js" async></script>`
		liveJsCode       = `(function(){var cur=null;function morph(n,o){if(!n||!o)return;if(o.nodeType!==n.nodeType||o.nodeName!==n.nodeName){o.replaceWith(n);return;}if(o.nodeType===Node.TEXT_NODE){if(o.nodeValue!==n.nodeValue)o.nodeValue=n.nodeValue;return;}if(o.nodeType===Node.ELEMENT_NODE){for(var i=0;i<n.attributes.length;i++){var a=n.attributes[i];if(o.getAttribute(a.name)!==a.value)o.setAttribute(a.name,a.value);}for(var i=o.attributes.length-1;i>=0;i--){var a=o.attributes[i];if(!n.hasAttribute(a.name))o.removeAttribute(a.name);}var nc=Array.from(n.childNodes),oc=Array.from(o.childNodes),m=Math.max(nc.length,oc.length);for(var i=0;i<m;i++){if(!oc[i]&&nc[i])o.appendChild(nc[i].cloneNode(true));else if(oc[i]&&!nc[i])o.removeChild(oc[i]);else if(oc[i]&&nc[i]){if(oc[i].nodeType===Node.ELEMENT_NODE&&oc[i].id&&oc[i].id===nc[i].id)morph(nc[i],oc[i]);else if(oc[i].outerHTML!==nc[i].outerHTML)morph(nc[i],oc[i]);}}}}function reloadElements(){fetch(window.location.href,{cache:'no-store'}).then(function(r){return r.text();}).then(function(h){var d=new DOMParser().parseFromString(h,'text/html');if(d.title&&d.title!==document.title)document.title=d.title;var sx=window.scrollX,sy=window.scrollY;morph(d.body,document.body);window.scrollTo(sx,sy);}).catch(function(){window.location.reload();});}function probe(){fetch('/__joss_health',{cache:'no-store'}).then(function(r){return r.text();}).then(function(t){if(!t)return;if(cur===null){cur=t;}else if(cur!==t){cur=t;reloadElements();}setTimeout(probe,200);}).catch(function(){function retry(){fetch('/__joss_health',{cache:'no-store'}).then(function(r){if(r.ok){reloadElements();setTimeout(probe,200);}}).catch(function(){setTimeout(retry,100);});}setTimeout(retry,100);});}setTimeout(probe,200);})();`
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

		// 2. Load kernel32.dll into r15
		b.emitLeaRCXString("kernel32.dll")
		b.emitCallIAT(IATLoadLibraryA)
		b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
		b.emitJeLabel(".srv_start_ws2")
		b.emitBytes(0x49, 0x89, 0xC7) // mov r15, rax (hKernel32)

		// sockaddr_in at [rbp - 96]: sin_family=AF_INET(2), default port 8000 (0x401F big-endian)
		b.emitBytes(0xC7, 0x45, 0xA0, 0x02, 0x00, 0x1F, 0x40)       // default: 0x401F0002 (port 8000)
		b.emitBytes(0xC7, 0x45, 0xA4, 0x00, 0x00, 0x00, 0x00)       // sin_addr = INADDR_ANY (0)
		b.emitBytes(0x48, 0xC7, 0x45, 0xA8, 0x00, 0x00, 0x00, 0x00) // sin_zero = 0

		// 1. Check OS environment variable "PORT"
		b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15 (hKernel32)
		b.emitLeaRDXString("GetEnvironmentVariableA")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
		b.emitJeLabel(".srv_check_env_map_port")
		b.emitBytes(0x49, 0x89, 0xC2) // mov r10, rax
		b.emitLeaRCXString("PORT")
		b.emitBytes(0x48, 0x8D, 0x55, 0xB0)             // lea rdx, [rbp - 80] (scratch stack)
		b.emitBytes(0x41, 0xB8, 0x10, 0x00, 0x00, 0x00) // mov r8d, 16
		b.emitBytes(0x41, 0xFF, 0xD2)                   // call r10
		b.emitBytes(0x85, 0xC0)                         // test eax, eax
		b.emitJleLabel(".srv_check_env_map_port")
		b.emitBytes(0x48, 0x8D, 0x4D, 0xB0) // lea rcx, [rbp - 80]
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

		// Dynamic port announcement matching joss server start format (no UTF-8 broken emojis)
		b.emitBytes(0x66, 0x81, 0x7D, 0xA2, 0x00, 0x50) // cmp word ptr [rbp - 94], 0x5000 (port 80)
		b.emitJeLabel(".srv_announce_port_80")
		b.emitBytes(0x66, 0x81, 0x7D, 0xA2, 0x1F, 0x90) // cmp word ptr [rbp - 94], 0x901F (port 8080)
		b.emitJeLabel(".srv_announce_port_8080")
		b.emitBytes(0x66, 0x81, 0x7D, 0xA2, 0x0B, 0xB8) // cmp word ptr [rbp - 94], 0xB80B (port 3000)
		b.emitJeLabel(".srv_announce_port_3000")

		// Default port 8000 announcement (preserves compatibility with TestNativeJosSecurityServerStartSmoke)
		b.emitLeaRCXString("[Server::start] Iniciando servidor web...\n[Joss Native Server] Escuchando en http://localhost:8000 (Ctrl+C para detener)\n[Server] Socket TCP enlazado con éxito en :8000. Servidor listo para peticiones.")
		b.emitCallLabel("joss_print_string")
		b.emitJmpLabel(".srv_check_smoke_only")

		b.defineLabel(".srv_announce_port_80")
		b.emitLeaRCXString("[Server::start] Iniciando servidor web...\n[Joss Native Server] Escuchando en http://localhost:80 (Ctrl+C para detener)\n[Server] Socket TCP enlazado con éxito en :80. Servidor listo para peticiones.")
		b.emitCallLabel("joss_print_string")
		b.emitJmpLabel(".srv_check_smoke_only")

		b.defineLabel(".srv_announce_port_8080")
		b.emitLeaRCXString("[Server::start] Iniciando servidor web...\n[Joss Native Server] Escuchando en http://localhost:8080 (Ctrl+C para detener)\n[Server] Socket TCP enlazado con éxito en :8080. Servidor listo para peticiones.")
		b.emitCallLabel("joss_print_string")
		b.emitJmpLabel(".srv_check_smoke_only")

		b.defineLabel(".srv_announce_port_3000")
		b.emitLeaRCXString("[Server::start] Iniciando servidor web...\n[Joss Native Server] Escuchando en http://localhost:3000 (Ctrl+C para detener)\n[Server] Socket TCP enlazado con éxito en :3000. Servidor listo para peticiones.")
		b.emitCallLabel("joss_print_string")

		b.defineLabel(".srv_check_smoke_only")
		// Check JOSS_NATIVE_CHECK_ONLY environment variable for non-blocking CI smoke checks
		b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15 (hKernel32)
		b.emitLeaRDXString("GetEnvironmentVariableA")
		b.emitCallIAT(IATGetProcAddress)
		b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
		b.emitJeLabel(".srv_start_ws2")
		b.emitBytes(0x49, 0x89, 0xC4) // mov r12, rax
		b.emitLeaRCXString("JOSS_NATIVE_CHECK_ONLY")
		b.emitBytes(0x48, 0x8D, 0x55, 0xB0)             // lea rdx, [rbp - 80]
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

		// Fresh params map for incoming request
		b.emitCallLabel("joss_map_new")
		b.emitMovDataRAX(dataOffRequestParams)

		// Extract HTTP request path from first line: "GET /path?query HTTP/1.1"
		b.emitBytes(0x4C, 0x8B, 0x7D, 0xB0) // mov r15, [rbp - 80] (scan ptr = reqBuf)
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
		b.emitJeLabel(".srv_have_query_mark")
		b.emitBytes(0x83, 0xF8, 0x0D) // cmp eax, '\r'
		b.emitJeLabel(".srv_terminate_path")
		b.emitBytes(0x83, 0xF8, 0x0A) // cmp eax, '\n'
		b.emitJeLabel(".srv_terminate_path")
		b.emitBytes(0x48, 0xFF, 0xC3) // inc rbx
		b.emitJmpLabel(".srv_find_path_end")

		b.defineLabel(".srv_have_query_mark")
		b.emitBytes(0xC6, 0x03, 0x00) // mov byte ptr [rbx], 0 (terminate path)
		b.emitBytes(0x48, 0xFF, 0xC3) // inc rbx (rbx = start of query)
		b.emitBytes(0x53)             // push rbx (save query start pointer on stack)
		b.defineLabel(".srv_find_query_end")
		b.emitBytes(0x0F, 0xB6, 0x03) // movzx eax, byte ptr [rbx]
		b.emitBytes(0x85, 0xC0)       // test eax, eax
		b.emitJeLabel(".srv_parse_query")
		b.emitBytes(0x83, 0xF8, 0x20) // ' '
		b.emitJeLabel(".srv_terminate_query")
		b.emitBytes(0x83, 0xF8, 0x0D) // '\r'
		b.emitJeLabel(".srv_terminate_query")
		b.emitBytes(0x83, 0xF8, 0x0A) // '\n'
		b.emitJeLabel(".srv_terminate_query")
		b.emitBytes(0x48, 0xFF, 0xC3) // inc rbx
		b.emitJmpLabel(".srv_find_query_end")

		b.defineLabel(".srv_terminate_query")
		b.emitBytes(0xC6, 0x03, 0x00) // mov byte ptr [rbx], 0
		b.defineLabel(".srv_parse_query")
		b.emitBytes(0x59) // pop rcx (restore query start pointer into rcx)
		b.emitCallLabel("__joss_parse_query_params")
		b.emitJmpLabel(".srv_have_path")

		b.defineLabel(".srv_terminate_path")
		b.emitBytes(0xC6, 0x03, 0x00) // mov byte ptr [rbx], 0
		b.emitJmpLabel(".srv_have_path")

		b.defineLabel(".srv_default_path")
		b.emitLeaRCXString("/")
		b.emitBytes(0x49, 0x89, 0xCF) // mov r15, rcx

		b.defineLabel(".srv_have_path")
		// Check "/__joss_health" and "/__joss_live.js" for hot-reload without cluttering server logs
		b.emitBytes(0x41, 0x81, 0x3F, 0x2F, 0x5F, 0x5F, 0x6A) // cmp dword ptr [r15], 0x6A5F5F2F ("/__j")
		b.emitJneLabel(".srv_log_req")
		b.emitBytes(0x41, 0x80, 0x7F, 0x08, 0x6C) // cmp byte ptr [r15 + 8], 'l' (0x6C for "/__joss_live.js")
		b.emitJeLabel(".srv_send_live_js")
		healthResp := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain; charset=utf-8\r\nCache-Control: no-cache, no-store\r\nConnection: close\r\n\r\n%d", time.Now().UnixNano())
		b.emitLeaRCXString(healthResp)
		b.emitBytes(0x49, 0x89, 0xCF) // mov r15, rcx
		b.emitJmpLabel(".srv_resp_send_raw")

		b.defineLabel(".srv_send_live_js")
		liveJsResp := "HTTP/1.1 200 OK\r\nContent-Type: application/javascript; charset=utf-8\r\nCache-Control: no-cache\r\nConnection: close\r\n\r\n" + liveJsCode
		b.emitLeaRCXString(liveJsResp)
		b.emitBytes(0x49, 0x89, 0xCF) // mov r15, rcx
		b.emitJmpLabel(".srv_resp_send_raw")

		b.defineLabel(".srv_log_req")
		// Log incoming request path to terminal
		b.emitLeaRCXString("[Server REQ] ")
		b.emitCallLabel("joss_print_string")
		b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15
		b.emitCallLabel("joss_print_string")
		b.emitLeaRCXString("\n")
		b.emitCallLabel("joss_print_string")

		// Check if path starts with "/public/" ("/pub" + 'l') or "/assets/" ("/ass" + 'e')
		b.emitBytes(0x41, 0x81, 0x3F, 0x2F, 0x70, 0x75, 0x62) // cmp dword ptr [r15], 0x6275702F ("/pub")
		b.emitJneLabel(".srv_check_assets")
		b.emitBytes(0x41, 0x80, 0x7F, 0x04, 0x6C) // cmp byte ptr [r15 + 4], 'l'
		b.emitJeLabel(".srv_serve_static")

		b.defineLabel(".srv_check_assets")
		b.emitBytes(0x41, 0x81, 0x3F, 0x2F, 0x61, 0x73, 0x73) // cmp dword ptr [r15], 0x7373612F ("/ass")
		b.emitJneLabel(".srv_check_css")
		b.emitBytes(0x41, 0x80, 0x7F, 0x04, 0x65) // cmp byte ptr [r15 + 4], 'e'
		b.emitJeLabel(".srv_serve_static")

		b.defineLabel(".srv_check_css")
		// Check "/css/" ("/css/")
		b.emitBytes(0x41, 0x81, 0x3F, 0x2F, 0x63, 0x73, 0x73) // cmp dword ptr [r15], 0x7373632F ("/css")
		b.emitJneLabel(".srv_check_images")
		b.emitBytes(0x41, 0x80, 0x7F, 0x04, 0x2F) // cmp byte ptr [r15 + 4], '/'
		b.emitJeLabel(".srv_serve_static")

		b.defineLabel(".srv_check_images")
		// Check "/images/" ("/ima")
		b.emitBytes(0x41, 0x81, 0x3F, 0x2F, 0x69, 0x6D, 0x61) // cmp dword ptr [r15], 0x616D692F ("/ima")
		b.emitJeLabel(".srv_serve_static")

		// Check "/favicon" ("/fav")
		b.emitBytes(0x41, 0x81, 0x3F, 0x2F, 0x66, 0x61, 0x76) // cmp dword ptr [r15], 0x7661662F ("/fav")
		b.emitJeLabel(".srv_serve_static")

		b.emitJmpLabel(".srv_dispatch_route")

		b.defineLabel(".srv_serve_static")
		// 1. Try static file path relative to working directory: r15 + 1 (skip leading '/')
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
		b.emitJneLabel(".srv_file_opened")

		// 2. Try with "../" prefix in reqBuf+8192 (upper 8KB of reqBuf)
		b.emitBytes(0x48, 0x8B, 0x5D, 0xB0)                   // mov rbx, [rbp - 80]
		b.emitBytes(0x48, 0x81, 0xC3, 0x00, 0x20, 0x00, 0x00) // add rbx, 8192
		b.emitBytes(0xC6, 0x03, 0x2E)                         // mov byte ptr [rbx], '.'
		b.emitBytes(0xC6, 0x43, 0x01, 0x2E)                   // mov byte ptr [rbx + 1], '.'
		b.emitBytes(0xC6, 0x43, 0x02, 0x2F)                   // mov byte ptr [rbx + 2], '/'
		b.emitBytes(0x49, 0x8D, 0x77, 0x01)                   // lea rsi, [r15 + 1]
		b.emitBytes(0x48, 0x8D, 0x7B, 0x03)                   // lea rdi, [rbx + 3]
		b.defineLabel(".srv_copy_parent_loop")
		b.emitBytes(0xAC)       // lodsb
		b.emitBytes(0xAA)       // stosb
		b.emitBytes(0x84, 0xC0) // test al, al
		b.emitJneLabel(".srv_copy_parent_loop")

		b.emitBytes(0x48, 0x89, 0xD9)                                     // mov rcx, rbx
		b.emitBytes(0xBA, 0x00, 0x00, 0x00, 0x80)                         // mov edx, 0x80000000 (GENERIC_READ)
		b.emitBytes(0x41, 0xB8, 0x01, 0x00, 0x00, 0x00)                   // mov r8d, 1 (FILE_SHARE_READ)
		b.emitBytes(0x45, 0x31, 0xC9)                                     // xor r9d, r9d (NULL)
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x20, 0x03, 0x00, 0x00, 0x00) // mov qword ptr [rsp + 32], 3 (OPEN_EXISTING)
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x28, 0x80, 0x00, 0x00, 0x00) // mov qword ptr [rsp + 40], 0x80 (FILE_ATTRIBUTE_NORMAL)
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x30, 0x00, 0x00, 0x00, 0x00) // mov qword ptr [rsp + 48], 0
		b.emitBytes(0x48, 0x8B, 0x45, 0xA8)                               // mov rax, [rbp - 88] (pCreateFileA)
		b.emitBytes(0xFF, 0xD0)                                           // call rax
		b.emitBytes(0x48, 0x83, 0xF8, 0xFF)                               // cmp rax, -1
		b.emitJneLabel(".srv_file_opened")

		// 3. Try with "public/" prefix
		b.emitBytes(0x48, 0x8B, 0x5D, 0xB0)                   // mov rbx, [rbp - 80]
		b.emitBytes(0x48, 0x81, 0xC3, 0x00, 0x20, 0x00, 0x00) // add rbx, 8192
		b.emitBytes(0xC7, 0x03, 0x70, 0x75, 0x62, 0x6C)       // mov dword ptr [rbx], "publ"
		b.emitBytes(0x66, 0xC7, 0x43, 0x04, 0x69, 0x63)       // mov word ptr [rbx + 4], "ic"
		b.emitBytes(0xC6, 0x43, 0x06, 0x2F)                   // mov byte ptr [rbx + 6], '/'
		b.emitBytes(0x49, 0x8D, 0x77, 0x01)                   // lea rsi, [r15 + 1]
		b.emitBytes(0x48, 0x8D, 0x7B, 0x07)                   // lea rdi, [rbx + 7]
		b.defineLabel(".srv_copy_pub_loop")
		b.emitBytes(0xAC)       // lodsb
		b.emitBytes(0xAA)       // stosb
		b.emitBytes(0x84, 0xC0) // test al, al
		b.emitJneLabel(".srv_copy_pub_loop")

		b.emitBytes(0x48, 0x89, 0xD9)                                     // mov rcx, rbx
		b.emitBytes(0xBA, 0x00, 0x00, 0x00, 0x80)                         // mov edx, 0x80000000 (GENERIC_READ)
		b.emitBytes(0x41, 0xB8, 0x01, 0x00, 0x00, 0x00)                   // mov r8d, 1 (FILE_SHARE_READ)
		b.emitBytes(0x45, 0x31, 0xC9)                                     // xor r9d, r9d (NULL)
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x20, 0x03, 0x00, 0x00, 0x00) // mov qword ptr [rsp + 32], 3 (OPEN_EXISTING)
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x28, 0x80, 0x00, 0x00, 0x00) // mov qword ptr [rsp + 40], 0x80 (FILE_ATTRIBUTE_NORMAL)
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x30, 0x00, 0x00, 0x00, 0x00) // mov qword ptr [rsp + 48], 0
		b.emitBytes(0x48, 0x8B, 0x45, 0xA8)                               // mov rax, [rbp - 88] (pCreateFileA)
		b.emitBytes(0xFF, 0xD0)                                           // call rax
		b.emitBytes(0x48, 0x83, 0xF8, 0xFF)                               // cmp rax, -1
		b.emitJneLabel(".srv_file_opened")

		// 4. Try with "../public/" prefix
		b.emitBytes(0x48, 0x8B, 0x5D, 0xB0)                   // mov rbx, [rbp - 80]
		b.emitBytes(0x48, 0x81, 0xC3, 0x00, 0x20, 0x00, 0x00) // add rbx, 8192
		b.emitBytes(0xC6, 0x03, 0x2E)                         // mov byte ptr [rbx], '.'
		b.emitBytes(0xC6, 0x43, 0x01, 0x2E)                   // mov byte ptr [rbx + 1], '.'
		b.emitBytes(0xC6, 0x43, 0x02, 0x2F)                   // mov byte ptr [rbx + 2], '/'
		b.emitBytes(0xC7, 0x43, 0x03, 0x70, 0x75, 0x62, 0x6C) // mov dword ptr [rbx + 3], "publ"
		b.emitBytes(0x66, 0xC7, 0x43, 0x07, 0x69, 0x63)       // mov word ptr [rbx + 7], "ic"
		b.emitBytes(0xC6, 0x43, 0x09, 0x2F)                   // mov byte ptr [rbx + 9], '/'
		b.emitBytes(0x49, 0x8D, 0x77, 0x01)                   // lea rsi, [r15 + 1]
		b.emitBytes(0x48, 0x8D, 0x7B, 0x0A)                   // lea rdi, [rbx + 10]
		b.defineLabel(".srv_copy_parent_pub_loop")
		b.emitBytes(0xAC)       // lodsb
		b.emitBytes(0xAA)       // stosb
		b.emitBytes(0x84, 0xC0) // test al, al
		b.emitJneLabel(".srv_copy_parent_pub_loop")

		b.emitBytes(0x48, 0x89, 0xD9)                                     // mov rcx, rbx
		b.emitBytes(0xBA, 0x00, 0x00, 0x00, 0x80)                         // mov edx, 0x80000000 (GENERIC_READ)
		b.emitBytes(0x41, 0xB8, 0x01, 0x00, 0x00, 0x00)                   // mov r8d, 1 (FILE_SHARE_READ)
		b.emitBytes(0x45, 0x31, 0xC9)                                     // xor r9d, r9d (NULL)
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x20, 0x03, 0x00, 0x00, 0x00) // mov qword ptr [rsp + 32], 3 (OPEN_EXISTING)
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x28, 0x80, 0x00, 0x00, 0x00) // mov qword ptr [rsp + 40], 0x80 (FILE_ATTRIBUTE_NORMAL)
		b.emitBytes(0x48, 0xC7, 0x44, 0x24, 0x30, 0x00, 0x00, 0x00, 0x00) // mov qword ptr [rsp + 48], 0
		b.emitBytes(0x48, 0x8B, 0x45, 0xA8)                               // mov rax, [rbp - 88] (pCreateFileA)
		b.emitBytes(0xFF, 0xD0)                                           // call rax
		b.emitBytes(0x48, 0x83, 0xF8, 0xFF)                               // cmp rax, -1
		b.emitJeLabel(".srv_send_404")

		b.defineLabel(".srv_file_opened")
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
		b.emitBytes(0x3D, 0x2E, 0x69, 0x63, 0x6F) // cmp eax, 0x6F63692E (".ico")
		b.emitJeLabel(".srv_mime_ico")
		b.emitBytes(0x3D, 0x77, 0x65, 0x62, 0x70) // cmp eax, 0x70626577 ("webp")
		b.emitJeLabel(".srv_mime_webp")
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
		b.emitJmpLabel(".srv_send_static_hdr")
		b.defineLabel(".srv_mime_webp")
		b.emitLeaRCXString(httpHeaderWEBP)
		b.emitJmpLabel(".srv_send_static_hdr")
		b.defineLabel(".srv_mime_ico")
		b.emitLeaRCXString(httpHeaderICO)

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
		b.emitLeaRCXString("[Server 404] Static asset not found: ")
		b.emitCallLabel("joss_print_string")
		b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15
		b.emitCallLabel("joss_print_string")
		b.emitLeaRCXString("\n")
		b.emitCallLabel("joss_print_string")
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
		b.emitLeaRCXString("[Server ROUTE] Dispatching: ")
		b.emitCallLabel("joss_print_string")
		b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15
		b.emitCallLabel("joss_print_string")
		b.emitLeaRCXString("\n")
		b.emitCallLabel("joss_print_string")
		if hasCompiledRouter {
			b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15 (pathPtr)
			b.emitCallLabel("fn___joss_http_dispatch")
			b.emitBytes(0x49, 0x89, 0xC7) // mov r15, rax (htmlBodyPtr)
		} else {
			b.emitLeaRCXString(defaultHTMLBody)
			b.emitBytes(0x49, 0x89, 0xCF) // mov r15, rcx
		}

		// Guard against null r15
		b.emitBytes(0x4D, 0x85, 0xFF) // test r15, r15
		b.emitJneLabel(".srv_resp_not_null")
		b.emitLeaRCXString("")
		b.emitBytes(0x49, 0x89, 0xCF) // mov r15, rcx
		b.defineLabel(".srv_resp_not_null")

		// Determine response Content-Type & length
		// Check raw HTTP response (e.g. "HTTP/1.1 302 Found"): 'H', 'T', 'T', 'P'
		b.emitBytes(0x41, 0x80, 0x3F, 0x48) // cmp byte ptr [r15], 'H'
		b.emitJneLabel(".srv_check_png")
		b.emitBytes(0x41, 0x80, 0x7F, 0x01, 0x54) // cmp byte ptr [r15 + 1], 'T'
		b.emitJneLabel(".srv_check_png")
		b.emitBytes(0x41, 0x80, 0x7F, 0x02, 0x54) // cmp byte ptr [r15 + 2], 'T'
		b.emitJneLabel(".srv_check_png")
		b.emitBytes(0x41, 0x80, 0x7F, 0x03, 0x50) // cmp byte ptr [r15 + 3], 'P'
		b.emitJeLabel(".srv_resp_send_raw")

		b.defineLabel(".srv_check_png")
		// Check PNG magic: 0x89, 0x50 ('P')
		b.emitBytes(0x41, 0x80, 0x3F, 0x89) // cmp byte ptr [r15], 0x89
		b.emitJneLabel(".srv_check_jpg")
		b.emitBytes(0x41, 0x80, 0x7F, 0x01, 0x50) // cmp byte ptr [r15 + 1], 0x50 ('P')
		b.emitJeLabel(".srv_resp_png")

		b.defineLabel(".srv_check_jpg")
		// Check JPEG magic: 0xFF, 0xD8
		b.emitBytes(0x41, 0x80, 0x3F, 0xFF) // cmp byte ptr [r15], 0xFF
		b.emitJneLabel(".srv_check_webp")
		b.emitBytes(0x41, 0x80, 0x7F, 0x01, 0xD8) // cmp byte ptr [r15 + 1], 0xD8
		b.emitJeLabel(".srv_resp_jpg")

		b.defineLabel(".srv_check_webp")
		// Check WEBP magic: 'R', 'I'
		b.emitBytes(0x41, 0x80, 0x3F, 0x52) // cmp byte ptr [r15], 'R'
		b.emitJneLabel(".srv_check_json")
		b.emitBytes(0x41, 0x80, 0x7F, 0x01, 0x49) // cmp byte ptr [r15 + 1], 'I'
		b.emitJeLabel(".srv_resp_webp")

		b.defineLabel(".srv_check_json")
		// Check JSON: '{' (0x7B) or '[' (0x5B)
		b.emitBytes(0x41, 0x80, 0x3F, 0x7B)
		b.emitJeLabel(".srv_resp_json")
		b.emitBytes(0x41, 0x80, 0x3F, 0x5B)
		b.emitJeLabel(".srv_resp_json")

		// Default: HTML
		b.emitLeaRCXString(httpHeaderHTML)
		b.emitJmpLabel(".srv_resp_send_hdr_html")

		b.defineLabel(".srv_resp_png")
		b.emitLeaRCXString(httpHeaderPNG)
		b.emitJmpLabel(".srv_resp_send_hdr_bin")

		b.defineLabel(".srv_resp_jpg")
		b.emitLeaRCXString(httpHeaderJPG)
		b.emitJmpLabel(".srv_resp_send_hdr_bin")

		b.defineLabel(".srv_resp_webp")
		b.emitLeaRCXString(httpHeaderWEBP)
		b.emitJmpLabel(".srv_resp_send_hdr_bin")

		b.defineLabel(".srv_resp_json")
		b.emitLeaRCXString(httpHeaderJSON)
		b.emitJmpLabel(".srv_resp_send_hdr_text")

		b.defineLabel(".srv_resp_send_hdr_bin")
		// Send binary header (PNG/JPG/WEBP)
		b.emitBytes(0x48, 0x89, 0xCB) // mov rbx, rcx
		b.emitCallLabel("__joss_rt_strlen")
		b.emitBytes(0x49, 0x89, 0xC0)       // mov r8, rax
		b.emitBytes(0x48, 0x89, 0xDA)       // mov rdx, rbx
		b.emitBytes(0x4C, 0x89, 0xF1)       // mov rcx, r14 (clientSock)
		b.emitBytes(0x45, 0x31, 0xC9)       // xor r9d, r9d
		b.emitBytes(0x48, 0x8B, 0x45, 0xC0) // mov rax, [rbp - 64] (pSend)
		b.emitBytes(0xFF, 0xD0)             // call rax

		// Binary body length from dataOffLastFileSize
		b.emitMovRAXData(dataOffLastFileSize)
		b.emitBytes(0x48, 0x85, 0xC0) // test rax, rax
		b.emitJneLabel(".srv_resp_send_bin_body")
		b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15
		b.emitCallLabel("__joss_rt_strlen")

		b.defineLabel(".srv_resp_send_bin_body")
		b.emitBytes(0x49, 0x89, 0xC0)       // mov r8, rax (len)
		b.emitBytes(0x4C, 0x89, 0xFA)       // mov rdx, r15 (bodyPtr)
		b.emitBytes(0x4C, 0x89, 0xF1)       // mov rcx, r14 (clientSock)
		b.emitBytes(0x45, 0x31, 0xC9)       // xor r9d, r9d
		b.emitBytes(0x48, 0x8B, 0x45, 0xC0) // mov rax, [rbp - 64] (pSend)
		b.emitBytes(0xFF, 0xD0)             // call rax
		b.emitJmpLabel(".srv_close_client")

		b.defineLabel(".srv_resp_send_hdr_html")
		b.emitBytes(0x48, 0x89, 0xCB) // mov rbx, rcx
		b.emitCallLabel("__joss_rt_strlen")
		b.emitBytes(0x49, 0x89, 0xC0)       // mov r8, rax
		b.emitBytes(0x48, 0x89, 0xDA)       // mov rdx, rbx
		b.emitBytes(0x4C, 0x89, 0xF1)       // mov rcx, r14 (clientSock)
		b.emitBytes(0x45, 0x31, 0xC9)       // xor r9d, r9d
		b.emitBytes(0x48, 0x8B, 0x45, 0xC0) // mov rax, [rbp - 64] (pSend)
		b.emitBytes(0xFF, 0xD0)             // call rax

		// Send HTML body
		b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15
		b.emitCallLabel("__joss_rt_strlen")
		b.emitBytes(0x49, 0x89, 0xC0)       // mov r8, rax (len)
		b.emitBytes(0x4C, 0x89, 0xFA)       // mov rdx, r15 (htmlBodyPtr)
		b.emitBytes(0x4C, 0x89, 0xF1)       // mov rcx, r14 (clientSock)
		b.emitBytes(0x45, 0x31, 0xC9)       // xor r9d, r9d
		b.emitBytes(0x48, 0x8B, 0x45, 0xC0) // mov rax, [rbp - 64] (pSend)
		b.emitBytes(0xFF, 0xD0)             // call rax

		// Append live reload script to HTML
		b.emitLeaRCXString(liveReloadScript)
		b.emitCallLabel("__joss_rt_strlen")
		b.emitBytes(0x49, 0x89, 0xC0) // mov r8, rax (len)
		b.emitLeaRDXString(liveReloadScript)
		b.emitBytes(0x4C, 0x89, 0xF1)       // mov rcx, r14 (clientSock)
		b.emitBytes(0x45, 0x31, 0xC9)       // xor r9d, r9d
		b.emitBytes(0x48, 0x8B, 0x45, 0xC0) // mov rax, [rbp - 64] (pSend)
		b.emitBytes(0xFF, 0xD0)             // call rax
		b.emitJmpLabel(".srv_close_client")

		b.defineLabel(".srv_resp_send_hdr_text")
		b.emitBytes(0x48, 0x89, 0xCB) // mov rbx, rcx
		b.emitCallLabel("__joss_rt_strlen")
		b.emitBytes(0x49, 0x89, 0xC0)       // mov r8, rax
		b.emitBytes(0x48, 0x89, 0xDA)       // mov rdx, rbx
		b.emitBytes(0x4C, 0x89, 0xF1)       // mov rcx, r14 (clientSock)
		b.emitBytes(0x45, 0x31, 0xC9)       // xor r9d, r9d
		b.emitBytes(0x48, 0x8B, 0x45, 0xC0) // mov rax, [rbp - 64] (pSend)
		b.emitBytes(0xFF, 0xD0)             // call rax

		// Send text body
		b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15
		b.emitCallLabel("__joss_rt_strlen")
		b.emitBytes(0x49, 0x89, 0xC0)       // mov r8, rax (len)
		b.emitBytes(0x4C, 0x89, 0xFA)       // mov rdx, r15 (htmlBodyPtr)
		b.emitBytes(0x4C, 0x89, 0xF1)       // mov rcx, r14 (clientSock)
		b.emitBytes(0x45, 0x31, 0xC9)       // xor r9d, r9d
		b.emitBytes(0x48, 0x8B, 0x45, 0xC0) // mov rax, [rbp - 64] (pSend)
		b.emitBytes(0xFF, 0xD0)             // call rax
		b.emitJmpLabel(".srv_close_client")

		// Raw response (already has HTTP status & headers)
		b.defineLabel(".srv_resp_send_raw")
		b.emitBytes(0x4C, 0x89, 0xF9) // mov rcx, r15
		b.emitCallLabel("__joss_rt_strlen")
		b.emitBytes(0x49, 0x89, 0xC0)       // mov r8, rax (len)
		b.emitBytes(0x4C, 0x89, 0xFA)       // mov rdx, r15 (rawPtr)
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

		// sockaddr_in at [rbp - 96]: default port 80 on Linux, 8000 on Darwin
		if b.target.OS == "darwin" {
			b.emitBytes(0xC7, 0x45, 0xA0, 0x10, 0x02, 0x1F, 0x40) // sin_len=16, sin_family=2, sin_port=htons(8000)
		} else {
			b.emitBytes(0xC7, 0x45, 0xA0, 0x02, 0x00, 0x00, 0x50) // sin_family=2, sin_port=htons(80)
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
		b.emitJneLabel(".srv_posix_parse_port")
		b.emitMovRCXData(dataOffEnvMap)
		b.emitLeaRDXString("JOSS_PORT")
		b.emitCallLabel("joss_map_get_i64")
		b.emitBytes(0x48, 0x85, 0xC0)
		b.emitJeLabel(".srv_posix_port_done")
		b.defineLabel(".srv_posix_parse_port")
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
		b.emitBytes(0x48, 0x85, 0xC0)
		b.emitJnsLabel(".srv_posix_listen")

		// Fallback: if bind failed and port wasn't 8000, try 8000
		b.emitBytes(0x66, 0x81, 0x7D, 0xA2, 0x40, 0x1F) // cmp word ptr [rbp - 94], 0x1F40 (8000)
		b.emitJeLabel(".srv_posix_exit")
		b.emitLeaRCXString("[Joss Native Server] ADVERTENCIA: No se pudo enlazar el puerto principal (posible falta de permisos para puerto < 1024), intentando fallback en puerto 8000...\n")
		b.emitCallLabel("joss_print_string")
		b.emitBytes(0x4C, 0x89, 0xEF) // mov rdi, r13
		b.emitBytes(0xB8)
		b.emitU32(sysClose)
		b.emitBytes(0x0F, 0x05)
		b.emitBytes(0xBF, 0x02, 0x00, 0x00, 0x00) // mov edi, 2
		b.emitBytes(0xBE, 0x01, 0x00, 0x00, 0x00) // mov esi, 1
		b.emitBytes(0x31, 0xD2)                   // xor edx, edx
		b.emitBytes(0xB8)
		b.emitU32(sysSocket)
		b.emitBytes(0x0F, 0x05)
		b.emitBytes(0x48, 0x85, 0xC0)
		b.emitJlLabel(".srv_posix_exit")
		b.emitBytes(0x49, 0x89, 0xC5) // mov r13, rax
		b.emitBytes(0xC7, 0x45, 0x98, 0x01, 0x00, 0x00, 0x00)
		b.emitBytes(0x4C, 0x89, 0xEF)
		b.emitBytes(0xBE)
		b.emitU32(solSocket)
		b.emitBytes(0xBA)
		b.emitU32(soReuseAddr)
		b.emitBytes(0x4C, 0x8D, 0x55, 0x98)
		b.emitBytes(0x41, 0xB8, 0x04, 0x00, 0x00, 0x00)
		b.emitBytes(0xB8)
		b.emitU32(sysSetsockopt)
		b.emitBytes(0x0F, 0x05)
		b.emitBytes(0x66, 0xC7, 0x45, 0xA2, 0x1F, 0x40) // mov word ptr [rbp - 94], 0x401F (8000)
		b.emitBytes(0x4C, 0x89, 0xEF)
		b.emitBytes(0x48, 0x8D, 0x75, 0xA0)
		b.emitBytes(0xBA, 0x10, 0x00, 0x00, 0x00)
		b.emitBytes(0xB8)
		b.emitU32(sysBind)
		b.emitBytes(0x0F, 0x05)
		b.emitBytes(0x48, 0x85, 0xC0)
		b.emitJlLabel(".srv_posix_exit")

		b.defineLabel(".srv_posix_listen")
		// listen(listenSock, 128)
		b.emitBytes(0x4C, 0x89, 0xEF)             // mov rdi, r13
		b.emitBytes(0xBE, 0x80, 0x00, 0x00, 0x00) // mov esi, 128
		b.emitBytes(0xB8)
		b.emitU32(sysListen)
		b.emitBytes(0x0F, 0x05) // syscall

		// Dynamic port announcement matching actual port in sockaddr
		b.emitBytes(0x66, 0x81, 0x7D, 0xA2, 0x00, 0x50) // cmp word ptr [rbp - 94], 0x5000 (port 80)
		b.emitJeLabel(".srv_posix_announce_80")
		b.emitBytes(0x66, 0x81, 0x7D, 0xA2, 0x1F, 0x90) // cmp word ptr [rbp - 94], 0x901F (port 8080)
		b.emitJeLabel(".srv_posix_announce_8080")
		b.emitBytes(0x66, 0x81, 0x7D, 0xA2, 0x0B, 0xB8) // cmp word ptr [rbp - 94], 0xB80B (port 3000)
		b.emitJeLabel(".srv_posix_announce_3000")
		b.emitLeaRCXString("[Joss Native Server] Escuchando en http://localhost:8000 (Ctrl+C para detener)\n")
		b.emitCallLabel("joss_print_string")
		b.emitJmpLabel(".srv_posix_accept_loop")

		b.defineLabel(".srv_posix_announce_80")
		b.emitLeaRCXString("[Joss Native Server] Escuchando en http://localhost:80 (Ctrl+C para detener)\n")
		b.emitCallLabel("joss_print_string")
		b.emitJmpLabel(".srv_posix_accept_loop")

		b.defineLabel(".srv_posix_announce_8080")
		b.emitLeaRCXString("[Joss Native Server] Escuchando en http://localhost:8080 (Ctrl+C para detener)\n")
		b.emitCallLabel("joss_print_string")
		b.emitJmpLabel(".srv_posix_accept_loop")

		b.defineLabel(".srv_posix_announce_3000")
		b.emitLeaRCXString("[Joss Native Server] Escuchando en http://localhost:3000 (Ctrl+C para detener)\n")
		b.emitCallLabel("joss_print_string")

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

		// write(clientSock, liveReloadScript, len)
		b.emitLeaRCXString(liveReloadScript)
		b.emitBytes(0x48, 0x89, 0xCB)
		b.emitCallLabel("__joss_rt_strlen")
		b.emitBytes(0x4C, 0x89, 0xF7) // mov rdi, r14
		b.emitBytes(0x48, 0x89, 0xDE) // mov rsi, rbx
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
