package native_test

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	backend "github.com/jossecurity/joss/pkg/backend/native"
)

func TestNative_SitemapIndexNowAndSMTPParity(t *testing.T) {
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang/zig not available on PATH")
	}

	// Mock IndexNow HTTP endpoint
	var indexNowBody string
	idxSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		indexNowBody = string(b)
		w.WriteHeader(200)
	}))
	defer idxSrv.Close()

	// Mock TCP SMTP server
	smtpLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen mock smtp: %v", err)
	}
	defer smtpLn.Close()
	var smtpMu sync.Mutex
	var smtpData string
	go func() {
		conn, err := smtpLn.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = conn.Write([]byte("220 localhost ESMTP\r\n"))
		r := bufio.NewReader(conn)
		inData := false
		var sb strings.Builder
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				break
			}
			if inData {
				if line == ".\r\n" {
					inData = false
					smtpMu.Lock()
					smtpData = sb.String()
					smtpMu.Unlock()
					_, _ = conn.Write([]byte("250 OK\r\n"))
				} else {
					sb.WriteString(line)
				}
				continue
			}
			upper := strings.ToUpper(strings.TrimSpace(line))
			switch {
			case strings.HasPrefix(upper, "EHLO") || strings.HasPrefix(upper, "HELO"):
				_, _ = conn.Write([]byte("250 localhost\r\n"))
			case strings.HasPrefix(upper, "MAIL FROM:") || strings.HasPrefix(upper, "RCPT TO:"):
				_, _ = conn.Write([]byte("250 OK\r\n"))
			case upper == "DATA":
				inData = true
				_, _ = conn.Write([]byte("354 End data with <CR><LF>.<CR><LF>\r\n"))
			case upper == "QUIT":
				_, _ = conn.Write([]byte("221 Bye\r\n"))
				return
			default:
				_, _ = conn.Write([]byte("250 OK\r\n"))
			}
		}
	}()

	src := `
public class Main {
	Init main() {
		Sitemap::exclude(["/admin*", "/secret"])
		Sitemap::provider(func() {
			return ["/blog/hello", "/admin/panel", "/secret", {"loc": "/docs/intro", "priority": "0.9"}]
		})
		string $xml = Sitemap::generate("https://joss.dev")
		print("XML_OUT:" . $xml)
		print("XML_HAS_BLOG:" . strval(str_contains($xml, "https://joss.dev/blog/hello")))
		print("XML_HAS_DOCS:" . strval(str_contains($xml, "https://joss.dev/docs/intro")))
		print("XML_EXCL_ADMIN:" . strval(!str_contains($xml, "/admin/panel") && !str_contains($xml, "/secret")))

		string $k = IndexNow::key()
		bool $idxOk = IndexNow::submit(["https://joss.dev/blog/hello"])
		print("IDX_KEY:" . $k . "|OK:" . strval($idxOk))

		bool $mailOk = SmtpClient::send("user@joss.dev", "Hola Native", "Cuerpo SMTP C11")
		print("SMTP_OK:" . strval($mailOk))
	}
}
`
	exe := buildNativeTestServerExe(t, src, "espiral5_services")
	smtpPort := smtpLn.Addr().(*net.TCPAddr).Port
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(),
		"INDEXNOW_KEY=key12345",
		"INDEXNOW_ENDPOINT="+idxSrv.URL,
		"APP_URL=https://joss.dev",
		"MAIL_HOST=127.0.0.1",
		fmt.Sprintf("MAIL_PORT=%d", smtpPort),
		"MAIL_FROM_ADDRESS=bot@joss.dev",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("espiral5_services failed: %v\noutput:\n%s", err, string(out))
	}
	got := string(out)
	for _, want := range []string{
		"XML_HAS_BLOG:true",
		"XML_HAS_DOCS:true",
		"XML_EXCL_ADMIN:true",
		"IDX_KEY:key12345|OK:true",
		"SMTP_OK:true",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in output:\n%s", want, got)
		}
	}
	if !strings.Contains(indexNowBody, `"key":"key12345"`) || !strings.Contains(indexNowBody, "https://joss.dev/blog/hello") {
		t.Fatalf("unexpected IndexNow payload: %s", indexNowBody)
	}
	smtpMu.Lock()
	sd := smtpData
	smtpMu.Unlock()
	if !strings.Contains(sd, "Subject: Hola Native") || !strings.Contains(sd, "Cuerpo SMTP C11") {
		t.Fatalf("unexpected SMTP body: %s", sd)
	}
}

func wsHandshakeClient(t *testing.T, addr string, path string, secKey string) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	req := fmt.Sprintf(
		"GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n",
		path, addr, secKey,
	)
	if _, err := io.WriteString(conn, req); err != nil {
		_ = conn.Close()
		t.Fatalf("write handshake: %v", err)
	}

	r := bufio.NewReader(conn)
	statusLine, err := r.ReadString('\n')
	if err != nil {
		_ = conn.Close()
		t.Fatalf("read handshake status: %v", err)
	}
	if !strings.Contains(statusLine, "101 Switching Protocols") {
		_ = conn.Close()
		t.Fatalf("expected 101 Switching Protocols, got %q", statusLine)
	}

	headers := make(map[string]string)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			_ = conn.Close()
			t.Fatalf("read handshake header: %v", err)
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			break
		}
		if idx := strings.IndexByte(trimmed, ':'); idx > 0 {
			k := strings.ToLower(strings.TrimSpace(trimmed[:idx]))
			v := strings.TrimSpace(trimmed[idx+1:])
			headers[k] = v
		}
	}

	sum := sha1.Sum([]byte(secKey + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	wantAccept := base64.StdEncoding.EncodeToString(sum[:])
	if got := headers["sec-websocket-accept"]; got != wantAccept {
		_ = conn.Close()
		t.Fatalf("Sec-WebSocket-Accept = %q, want %q", got, wantAccept)
	}
	return conn, r
}

func wsWriteMaskedFrame(w io.Writer, opcode byte, payload []byte) error {
	maskKey := [4]byte{0x37, 0xFA, 0x21, 0x3D}
	var hdr []byte
	first := byte(0x80 | (opcode & 0x0F))
	plen := len(payload)
	switch {
	case plen < 126:
		hdr = []byte{first, byte(0x80 | plen)}
	case plen <= 65535:
		hdr = []byte{first, 0x80 | 126, byte(plen >> 8), byte(plen)}
	default:
		hdr = make([]byte, 10)
		hdr[0] = first
		hdr[1] = 0x80 | 127
		binary.BigEndian.PutUint64(hdr[2:10], uint64(plen))
	}
	frame := make([]byte, 0, len(hdr)+4+plen)
	frame = append(frame, hdr...)
	frame = append(frame, maskKey[:]...)
	for i, b := range payload {
		frame = append(frame, b^maskKey[i&3])
	}
	_, err := w.Write(frame)
	return err
}

func wsReadFrame(r *bufio.Reader) (byte, []byte, error) {
	var hdr [2]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return 0, nil, err
	}
	opcode := hdr[0] & 0x0F
	masked := (hdr[1] & 0x80) != 0
	plen := uint64(hdr[1] & 0x7F)
	if plen == 126 {
		var ext [2]byte
		if _, err := io.ReadFull(r, ext[:]); err != nil {
			return 0, nil, err
		}
		plen = uint64(binary.BigEndian.Uint16(ext[:]))
	} else if plen == 127 {
		var ext [8]byte
		if _, err := io.ReadFull(r, ext[:]); err != nil {
			return 0, nil, err
		}
		plen = binary.BigEndian.Uint64(ext[:])
	}
	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(r, mask[:]); err != nil {
			return 0, nil, err
		}
	}
	payload := make([]byte, plen)
	if plen > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return 0, nil, err
		}
		if masked {
			for i := range payload {
				payload[i] ^= mask[i&3]
			}
		}
	}
	return opcode, payload, nil
}

func TestNative_WebSocketRFC6455AndCronParity(t *testing.T) {
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang/zig not available on PATH")
	}

	t.Run("rfc6455_handshake_masked_ping_pong_and_live_pubsub", func(t *testing.T) {
		wsSrc := `
public class Main {
	public static func handleWsClient(Socket $client): void {
		guard ($client != null) : {
			return
		}
		mixed $raw = __joss_http_read_request($client)
		string $rawText = strval($raw)
		(strlen($rawText) == 0) ? {
			$client->close()
			return
		}
		var $lines = explode("\r\n", $rawText)
		guard (count($lines) > 0) : {
			$client->close()
			return
		}
		var $reqLine = explode(" ", $lines[0])
		string $path = (count($reqLine) >= 2) ? $reqLine[1] : "/"
		var $pathParts = explode("?", $path)
		(count($pathParts) > 0) ? {
			$path = $pathParts[0]
		}
		var $upgradeHeader = ""
		var $connectionHeader = ""
		var $secKey = ""
		var $idx = 1
		var $numLines = count($lines)
		while ($idx < $numLines) {
			var $line = $lines[$idx]
			(strlen($line) == 0) ? {
				break
			}
			var $lowerLine = strtolower($line)
			(str_contains($lowerLine, "upgrade:")) ? {
				var $uParts = explode(":", $line)
				(count($uParts) >= 2) ? {
					$upgradeHeader = trim($uParts[1])
				}
			}
			(str_contains($lowerLine, "connection:")) ? {
				var $cParts = explode(":", $line)
				(count($cParts) >= 2) ? {
					$connectionHeader = trim($cParts[1])
				}
			}
			(str_contains($lowerLine, "sec-websocket-key:")) ? {
				var $kParts = explode(":", $line)
				(count($kParts) >= 2) ? {
					$secKey = trim($kParts[1])
				}
			}
			$idx++
		}
		var $isWebSocket = WebSocketProtocol::isUpgradeRequest($upgradeHeader, $connectionHeader)
		($isWebSocket && $secKey != "") ? {
			$client->send(WebSocketProtocol::handshakeResponse($secKey))
			map $wsSession = {}
			__joss_http_dispatch("__websocket__", $path, $client, $wsSession, "")
			return
		}
		$client->send("HTTP/1.1 400 Bad Request\r\nConnection: close\r\n\r\n")
		$client->close()
	}

	Init main() {
		async {
			int $tries = 0
			while ($tries < 120) {
				int $cnt = WebSocket::subscriberCount("/ws")
				($cnt >= 2) ? {
					print("WS_LIVE_SUBS:" . strval($cnt))
					int $pub = WebSocket::publish("/ws", "server-live-broadcast")
					print("WS_LIVE_PUB:" . strval($pub))
					break
				}
				usleep(50000)
				$tries++
			}
		}
		string $portStr = env("PORT", "8080")
		var $server = Socket::listen("0.0.0.0", $portStr)
		guard ($server != null) : {
			return
		}
		print("[Joss Native Server] Escuchando en http://localhost:" . $portStr . "\n")
		__joss_http_serve_concurrent($server, func(Socket $client): void {
			Main::handleWsClient($client)
		})
	}
}
`
		wsExe := buildNativeTestServerExe(t, wsSrc, "ws_rfc6455_live")
		port := allocFreePort(t)
		cmd, logs := startBuiltServer(t, wsExe, port, nil)
		serverStopped := false
		t.Cleanup(func() {
			if !serverStopped && cmd != nil && cmd.Process != nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
		})

		addr := fmt.Sprintf("127.0.0.1:%d", port)
		// RFC 6455 Section 4.2.2 standard test key -> Sec-WebSocket-Accept: s3pPLMBiTxaQ9kYGzzhZRbK+xOo=
		c1, r1 := wsHandshakeClient(t, addr, "/ws", "dGhlIHNhbXBsZSBub25jZQ==")
		defer c1.Close()
		c2, r2 := wsHandshakeClient(t, addr, "/ws", "x3JJHMbDL1EzLkh9GBhXDw==")
		defer c2.Close()

		// Wait for server async publisher to detect 2 subscribers and broadcast "server-live-broadcast"
		readExpected := func(r *bufio.Reader, label string, wantOp byte, wantText string) {
			t.Helper()
			for i := 0; i < 8; i++ {
				op, data, err := wsReadFrame(r)
				if err != nil {
					t.Fatalf("%s read frame %d: %v", label, i, err)
				}
				if op == wantOp && string(data) == wantText {
					return
				}
			}
			t.Fatalf("%s did not receive expected frame op=0x%x text=%q", label, wantOp, wantText)
		}

		readExpected(r1, "c1-live-broadcast", 0x01, "server-live-broadcast")
		readExpected(r2, "c2-live-broadcast", 0x01, "server-live-broadcast")

		// Send masked RFC 6455 PING -> expect unmasked PONG with identical payload
		if err := wsWriteMaskedFrame(c1, 0x09, []byte("ping-rfc6455")); err != nil {
			t.Fatalf("c1 write PING: %v", err)
		}
		readExpected(r1, "c1-pong", 0x0A, "ping-rfc6455")

		// Send masked RFC 6455 TEXT frame -> expect broadcast to both c1 and c2
		if err := wsWriteMaskedFrame(c1, 0x01, []byte("hello-from-c1")); err != nil {
			t.Fatalf("c1 write TEXT: %v", err)
		}
		readExpected(r1, "c1-echo-pubsub", 0x01, "hello-from-c1")
		readExpected(r2, "c2-recv-pubsub", 0x01, "hello-from-c1")

		// Send masked RFC 6455 CLOSE frame -> expect server CLOSE echo
		if err := wsWriteMaskedFrame(c1, 0x08, []byte{0x03, 0xE8}); err != nil {
			t.Fatalf("c1 write CLOSE: %v", err)
		}
		op, _, err := wsReadFrame(r1)
		if err != nil || op != 0x08 {
			t.Fatalf("c1 expected CLOSE (0x8), got op=0x%x err=%v", op, err)
		}

		_ = c1.Close()
		_ = c2.Close()
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			serverStopped = true
		}
		out := logs.String()
		if !strings.Contains(out, "WS_LIVE_SUBS:2") || !strings.Contains(out, "WS_LIVE_PUB:2") {
			t.Fatalf("expected WS_LIVE_SUBS:2 and WS_LIVE_PUB:2 in server output:\n%s", out)
		}
	})

	t.Run("cron_schedule_matching_and_nonmatching_with_ws_pubsub", func(t *testing.T) {
		cronSrc := `
public class Main {
	Init main() {
		map $cronState = {"matched": 0, "unmatched": 0}
		string $label = "MINUTE_JOB"

		Cron::schedule("* * * * *", func() {
			$cronState["matched"] = intval($cronState["matched"]) + 1
			print("CRON_MATCHED:" . $label)
		})
		Cron::schedule("0 0 1 1 *", func() {
			$cronState["unmatched"] = intval($cronState["unmatched"]) + 1
			print("CRON_UNMATCHED_FAIL")
		})

		int $before = WebSocket::subscriberCount("/alerts")
		int $sent = WebSocket::publish("/alerts", "alert-1")
		int $after = WebSocket::subscriberCount("/alerts")
		print("WS_OFFLINE:" . strval($before) . ":" . strval($sent) . ":" . strval($after))

		sleep(2)
		print("CRON_COUNTS:" . strval($cronState["matched"]) . ":" . strval($cronState["unmatched"]))
	}
}
`
		cronExe := buildNativeTestServerExe(t, cronSrc, "cron_ws_parity")
		outBytes, err := exec.Command(cronExe).CombinedOutput()
		if err != nil {
			t.Fatalf("cron_ws_parity failed: %v\noutput:\n%s", err, string(outBytes))
		}
		got := string(outBytes)
		if !strings.Contains(got, "WS_OFFLINE:0:1:1") {
			t.Fatalf("expected WS_OFFLINE:0:1:1 in output:\n%s", got)
		}
		if !strings.Contains(got, "CRON_MATCHED:MINUTE_JOB") {
			t.Fatalf("expected matching cron closure to execute in output:\n%s", got)
		}
		now := time.Now()
		isJan1Midnight := now.Month() == time.January && now.Day() == 1 && now.Hour() == 0 && now.Minute() == 0
		if !isJan1Midnight && strings.Contains(got, "CRON_UNMATCHED_FAIL") {
			t.Fatalf("non-matching cron closure (0 0 1 1 *) unexpectedly ran:\n%s", got)
		}
		var matchedCount, unmatchedCount int
		for _, line := range strings.Split(strings.ReplaceAll(got, "\r\n", "\n"), "\n") {
			if strings.HasPrefix(line, "CRON_COUNTS:") {
				parts := strings.Split(strings.TrimPrefix(line, "CRON_COUNTS:"), ":")
				if len(parts) == 2 {
					matchedCount, _ = strconv.Atoi(parts[0])
					unmatchedCount, _ = strconv.Atoi(parts[1])
				}
			}
		}
		if matchedCount < 1 {
			t.Fatalf("expected matchedCount >= 1, got %d\noutput:\n%s", matchedCount, got)
		}
		if !isJan1Midnight && unmatchedCount != 0 {
			t.Fatalf("expected unmatchedCount == 0, got %d\noutput:\n%s", unmatchedCount, got)
		}
	})
}
