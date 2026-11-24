package native_test

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	backend "github.com/jossecurity/joss/pkg/backend/native"
)

func startMockPGServer(t *testing.T, authMode string, user, pass string, dropFirstQuery bool) int {
	return startMockPGServerWithQueryHandler(t, authMode, user, pass, dropFirstQuery, nil)
}

func startMockPGServerWithQueryHandler(
	t *testing.T,
	authMode string,
	user, pass string,
	dropFirstQuery bool,
	onQuery func(sql string) (cols []string, rows [][]string),
) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen mock pg: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	var connCount atomic.Int32
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			cid := connCount.Add(1)
			go func(conn net.Conn, id int32) {
				defer conn.Close()
				// Read StartupMessage
				lenBuf := make([]byte, 4)
				if _, err := io.ReadFull(conn, lenBuf); err != nil {
					return
				}
				mlen := binary.BigEndian.Uint32(lenBuf)
				if mlen < 4 || mlen > 4096 {
					return
				}
				startup := make([]byte, mlen-4)
				if _, err := io.ReadFull(conn, startup); err != nil {
					return
				}
				if mlen == 8 && binary.BigEndian.Uint32(startup) == 80877103 {
					_, _ = conn.Write([]byte{'N'})
					if _, err := io.ReadFull(conn, lenBuf); err != nil {
						return
					}
					mlen = binary.BigEndian.Uint32(lenBuf)
					if mlen < 4 || mlen > 4096 {
						return
					}
					startup = make([]byte, mlen-4)
					if _, err := io.ReadFull(conn, startup); err != nil {
						return
					}
				}

				switch authMode {
				case "md5":
					salt := []byte{0x11, 0x22, 0x33, 0x44}
					msg := make([]byte, 13)
					msg[0] = 'R'
					binary.BigEndian.PutUint32(msg[1:5], 12)
					binary.BigEndian.PutUint32(msg[5:9], 5)
					copy(msg[9:13], salt)
					_, _ = conn.Write(msg)

					hdr := make([]byte, 5)
					if _, err := io.ReadFull(conn, hdr); err != nil || hdr[0] != 'p' {
						return
					}
					plen := binary.BigEndian.Uint32(hdr[1:5])
					pbody := make([]byte, plen-4)
					if _, err := io.ReadFull(conn, pbody); err != nil {
						return
					}
					inner := md5.Sum([]byte(pass + user))
					innerHex := hex.EncodeToString(inner[:])
					outer := md5.Sum(append([]byte(innerHex), salt...))
					want := "md5" + hex.EncodeToString(outer[:])
					got := strings.TrimRight(string(pbody), "\x00")
					if got != want {
						return
					}

				case "scram":
					// Send AuthenticationSASL (10)
					mech := "SCRAM-SHA-256\x00\x00"
					saslMsg := make([]byte, 9+len(mech))
					saslMsg[0] = 'R'
					binary.BigEndian.PutUint32(saslMsg[1:5], uint32(8+len(mech)))
					binary.BigEndian.PutUint32(saslMsg[5:9], 10)
					copy(saslMsg[9:], mech)
					_, _ = conn.Write(saslMsg)

					// Read SASLInitialResponse ('p')
					hdr := make([]byte, 5)
					if _, err := io.ReadFull(conn, hdr); err != nil || hdr[0] != 'p' {
						return
					}
					plen := binary.BigEndian.Uint32(hdr[1:5])
					pbody := make([]byte, plen-4)
					if _, err := io.ReadFull(conn, pbody); err != nil {
						return
					}
					nul := strings.IndexByte(string(pbody), 0)
					if nul < 0 || nul+5 > len(pbody) {
						return
					}
					cFirst := string(pbody[nul+5:])
					cFirstBare := strings.TrimPrefix(cFirst, "n,,")
					var cNonce string
					for _, part := range strings.Split(cFirstBare, ",") {
						if strings.HasPrefix(part, "r=") {
							cNonce = strings.TrimPrefix(part, "r=")
						}
					}
					sNonce := cNonce + "srvnonce"
					salt := []byte("testsalt1234")
					sFirst := fmt.Sprintf("r=%s,s=dGVzdHNhbHQxMjM0,i=4096", sNonce)

					// Send AuthenticationSASLContinue (11)
					contMsg := make([]byte, 9+len(sFirst))
					contMsg[0] = 'R'
					binary.BigEndian.PutUint32(contMsg[1:5], uint32(8+len(sFirst)))
					binary.BigEndian.PutUint32(contMsg[5:9], 11)
					copy(contMsg[9:], sFirst)
					_, _ = conn.Write(contMsg)

					// Read SASLResponse ('p')
					if _, err := io.ReadFull(conn, hdr); err != nil || hdr[0] != 'p' {
						return
					}
					plen = binary.BigEndian.Uint32(hdr[1:5])
					pbody = make([]byte, plen-4)
					if _, err := io.ReadFull(conn, pbody); err != nil {
						return
					}
					// Compute ServerSignature for AuthenticationSASLFinal (12)
					salted := pbkdf2SHA256([]byte(pass), salt, 4096, 32)
					mac := hmac.New(sha256.New, salted)
					mac.Write([]byte("Server Key"))
					serverKey := mac.Sum(nil)
					authMsg := cFirstBare + "," + sFirst + ",c=biws,r=" + sNonce
					mac2 := hmac.New(sha256.New, serverKey)
					mac2.Write([]byte(authMsg))
					// Base64 encode
					const b64 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
					sig := encodeB64(mac2.Sum(nil), b64)
					sFinal := "v=" + sig
					finMsg := make([]byte, 9+len(sFinal))
					finMsg[0] = 'R'
					binary.BigEndian.PutUint32(finMsg[1:5], uint32(8+len(sFinal)))
					binary.BigEndian.PutUint32(finMsg[5:9], 12)
					copy(finMsg[9:], sFinal)
					_, _ = conn.Write(finMsg)
				}

				// Send AuthenticationOk (0) + ReadyForQuery ('Z')
				okMsg := []byte{'R', 0, 0, 0, 8, 0, 0, 0, 0}
				rfq := []byte{'Z', 0, 0, 0, 5, 'I'}
				_, _ = conn.Write(okMsg)
				_, _ = conn.Write(rfq)

				// Query loop (supports both Simple Query 'Q' and Extended Query P/B/D/E/S)
				writeRowDesc := func(cols []string) {
					tBody := make([]byte, 2)
					binary.BigEndian.PutUint16(tBody[0:2], uint16(len(cols)))
					for _, col := range cols {
						tBody = append(tBody, []byte(col)...)
						tBody = append(tBody, 0)
						tBody = append(tBody, make([]byte, 18)...)
					}
					tMsg := make([]byte, 5+len(tBody))
					tMsg[0] = 'T'
					binary.BigEndian.PutUint32(tMsg[1:5], uint32(4+len(tBody)))
					copy(tMsg[5:], tBody)
					_, _ = conn.Write(tMsg)
				}
				writeDataRows := func(cols []string, rows [][]string) {
					for _, row := range rows {
						dBody := make([]byte, 2)
						binary.BigEndian.PutUint16(dBody[0:2], uint16(len(cols)))
						for ci := range cols {
							val := ""
							if ci < len(row) {
								val = row[ci]
							}
							var lb [4]byte
							binary.BigEndian.PutUint32(lb[:], uint32(len(val)))
							dBody = append(dBody, lb[:]...)
							dBody = append(dBody, []byte(val)...)
						}
						dMsg := make([]byte, 5+len(dBody))
						dMsg[0] = 'D'
						binary.BigEndian.PutUint32(dMsg[1:5], uint32(4+len(dBody)))
						copy(dMsg[5:], dBody)
						_, _ = conn.Write(dMsg)
					}
				}
				writeCmdComplete := func() {
					cmdTag := "OK\x00"
					cMsg := make([]byte, 5+len(cmdTag))
					cMsg[0] = 'C'
					binary.BigEndian.PutUint32(cMsg[1:5], uint32(4+len(cmdTag)))
					copy(cMsg[5:], cmdTag)
					_, _ = conn.Write(cMsg)
				}
				var pendingCols []string
				var pendingRows [][]string
				for {
					hdr := make([]byte, 5)
					if _, err := io.ReadFull(conn, hdr); err != nil {
						return
					}
					qlen := binary.BigEndian.Uint32(hdr[1:5])
					qbody := make([]byte, qlen-4)
					if _, err := io.ReadFull(conn, qbody); err != nil {
						return
					}
					switch hdr[0] {
					case 'Q':
						if dropFirstQuery && id == 1 {
							return
						}
						sqlText := strings.TrimRight(string(qbody), "\x00")
						cols := []string{"ok"}
						rows := [][]string{{"1"}}
						if onQuery != nil {
							cols, rows = onQuery(sqlText)
						}
						if len(cols) > 0 {
							writeRowDesc(cols)
							writeDataRows(cols, rows)
						}
						writeCmdComplete()
						_, _ = conn.Write(rfq)
					case 'P':
						if dropFirstQuery && id == 1 {
							return
						}
						parts := strings.SplitN(string(qbody), "\x00", 3)
						sqlText := ""
						if len(parts) >= 2 {
							sqlText = parts[1]
						}
						pendingCols = []string{"ok"}
						pendingRows = [][]string{{"1"}}
						if onQuery != nil {
							pendingCols, pendingRows = onQuery(sqlText)
						}
						_, _ = conn.Write([]byte{'1', 0, 0, 0, 4}) // ParseComplete
					case 'B':
						_, _ = conn.Write([]byte{'2', 0, 0, 0, 4}) // BindComplete
					case 'D':
						if len(pendingCols) > 0 {
							writeRowDesc(pendingCols)
						} else {
							_, _ = conn.Write([]byte{'n', 0, 0, 0, 4}) // NoData
						}
					case 'E':
						if len(pendingCols) > 0 {
							writeDataRows(pendingCols, pendingRows)
						}
						writeCmdComplete()
					case 'S':
						_, _ = conn.Write(rfq)
					case 'X':
						return
					}
				}
			}(c, cid)
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port
}

func pbkdf2SHA256(password, salt []byte, iter, keyLen int) []byte {
	prf := func(k, d []byte) []byte {
		h := hmac.New(sha256.New, k)
		h.Write(d)
		return h.Sum(nil)
	}
	numBlocks := (keyLen + 31) / 32
	var dk []byte
	for block := 1; block <= numBlocks; block++ {
		u := make([]byte, len(salt)+4)
		copy(u, salt)
		binary.BigEndian.PutUint32(u[len(salt):], uint32(block))
		u = prf(password, u)
		t := make([]byte, len(u))
		copy(t, u)
		for i := 2; i <= iter; i++ {
			u = prf(password, u)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		dk = append(dk, t...)
	}
	return dk[:keyLen]
}

func encodeB64(in []byte, tbl string) string {
	var out strings.Builder
	for i := 0; i < len(in); i += 3 {
		v := uint32(in[i]) << 16
		if i+1 < len(in) {
			v |= uint32(in[i+1]) << 8
		}
		if i+2 < len(in) {
			v |= uint32(in[i+2])
		}
		out.WriteByte(tbl[(v>>18)&0x3F])
		out.WriteByte(tbl[(v>>12)&0x3F])
		if i+1 < len(in) {
			out.WriteByte(tbl[(v>>6)&0x3F])
		} else {
			out.WriteByte('=')
		}
		if i+2 < len(in) {
			out.WriteByte(tbl[v&0x3F])
		} else {
			out.WriteByte('=')
		}
	}
	return out.String()
}

func TestNative_DBWireAuthAndReconnect(t *testing.T) {
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang/zig not available on PATH")
	}

	src := `
public class Main {
	Init main() {
		bool $ok = __joss_db_is_connected()
		var $rows = __joss_db_query("SELECT 1 AS ok", [])
		print("CONN:" . strval($ok) . "|ROWS:" . strval(count($rows)))
	}
}
`
	exe := buildNativeTestServerExe(t, src, "pg_wire_test")

	for _, mode := range []string{"md5", "scram"} {
		t.Run("pg_"+mode+"_auth_and_reconnect", func(t *testing.T) {
			port := startMockPGServer(t, mode, "pguser", "pgpass123", true)
			cmd := exec.Command(exe)
			cmd.Env = append(os.Environ(),
				"DB=pgsql",
				"DB_HOST=127.0.0.1",
				fmt.Sprintf("DB_PORT=%d", port),
				"DB_USERNAME=pguser",
				"DB_PASSWORD=pgpass123",
				"DB_DATABASE=appdb",
			)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("pg %s failed: %v\noutput:\n%s", mode, err, string(out))
			}
			if !strings.Contains(string(out), "CONN:true|ROWS:1") {
				t.Fatalf("pg %s expected CONN:true|ROWS:1, got:\n%s", mode, string(out))
			}
		})
	}
}

func startMockPGSchemaServer(t *testing.T, user, pass string) (int, *[]string, *sync.Mutex) {
	t.Helper()
	var mu sync.Mutex
	tables := map[string]map[string]bool{}
	var sqlLog []string

	extractQuotedAfter := func(sql, marker string) string {
		idx := strings.Index(sql, marker)
		if idx < 0 {
			return ""
		}
		rest := sql[idx+len(marker):]
		end := strings.IndexByte(rest, '\'')
		if end < 0 {
			return ""
		}
		return rest[:end]
	}

	port := startMockPGServerWithQueryHandler(t, "md5", user, pass, false, func(sql string) ([]string, [][]string) {
		mu.Lock()
		defer mu.Unlock()
		sqlLog = append(sqlLog, sql)

		switch {
		case strings.HasPrefix(sql, "CREATE TABLE IF NOT EXISTS "):
			rest := strings.TrimPrefix(sql, "CREATE TABLE IF NOT EXISTS ")
			openParen := strings.IndexByte(rest, '(')
			closeParen := strings.LastIndexByte(rest, ')')
			if openParen > 0 && closeParen > openParen {
				tbl := strings.TrimSpace(rest[:openParen])
				colsPart := rest[openParen+1 : closeParen]
				colSet := map[string]bool{}
				for _, def := range strings.Split(colsPart, ",") {
					fields := strings.Fields(strings.TrimSpace(def))
					if len(fields) > 0 {
						colSet[fields[0]] = true
					}
				}
				tables[tbl] = colSet
			}
			return nil, nil

		case strings.HasPrefix(sql, "ALTER TABLE ") && strings.Contains(sql, " ADD COLUMN "):
			rest := strings.TrimPrefix(sql, "ALTER TABLE ")
			parts := strings.SplitN(rest, " ADD COLUMN ", 2)
			if len(parts) == 2 {
				tbl := strings.TrimSpace(parts[0])
				fields := strings.Fields(strings.TrimSpace(parts[1]))
				if len(fields) > 0 {
					if tables[tbl] == nil {
						tables[tbl] = map[string]bool{}
					}
					tables[tbl][fields[0]] = true
				}
			}
			return nil, nil

		case strings.HasPrefix(sql, "ALTER TABLE ") && strings.Contains(sql, " RENAME TO "):
			rest := strings.TrimPrefix(sql, "ALTER TABLE ")
			parts := strings.SplitN(rest, " RENAME TO ", 2)
			if len(parts) == 2 {
				from := strings.TrimSpace(parts[0])
				to := strings.TrimSpace(parts[1])
				if cols, ok := tables[from]; ok {
					tables[to] = cols
					delete(tables, from)
				}
			}
			return nil, nil

		case strings.HasPrefix(sql, "DROP TABLE IF EXISTS "):
			tbl := strings.TrimSpace(strings.TrimPrefix(sql, "DROP TABLE IF EXISTS "))
			delete(tables, tbl)
			return nil, nil

		case strings.Contains(sql, "information_schema.tables"):
			tbl := extractQuotedAfter(sql, "table_name='")
			if _, ok := tables[tbl]; ok {
				return []string{"table_name"}, [][]string{{tbl}}
			}
			return []string{"table_name"}, nil

		case strings.Contains(sql, "information_schema.columns"):
			tbl := extractQuotedAfter(sql, "table_name='")
			col := extractQuotedAfter(sql, "column_name='")
			if cols, ok := tables[tbl]; ok && cols[col] {
				return []string{"column_name"}, [][]string{{col}}
			}
			return []string{"column_name"}, nil
		}
		return []string{"ok"}, [][]string{{"1"}}
	})

	return port, &sqlLog, &mu
}

func TestNative_SchemaDDLAndClosureAutoBackend(t *testing.T) {
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang/zig not available on PATH")
	}

	jossExe := buildJossCLIBinaryForTest(t)

	t.Run("closure_lexical_capture_auto_backend_vs_anterior", func(t *testing.T) {
		workDir := t.TempDir()
		src := `public class CounterState {
	public int $val = 0
}

public class ClosureFactory {
	public static func makeAdder(int $base, string $tag): mixed {
		return func(int $delta): string {
			return $tag . ":" . strval($base + $delta)
		}
	}
}

public class Main {
	Init main() {
		int $base = 100
		string $prefix = "ACC"
		CounterState $box = new CounterState()
		$box->val = 5
		map $counters = {"hits": 0}

		var $adder = ClosureFactory::makeAdder($base, "SUM")
		var $step = func(int $inc): string {
			$box->val = $box->val + $inc
			$counters["hits"] = intval($counters["hits"]) + 1
			int $localCopy = $base + $box->val
			return $prefix . ":" . strval($box->val) . ":" . strval($counters["hits"]) . ":" . strval($localCopy)
		}

		$base = 200
		int $x = 10
		var $reassign = func(int $d): int {
			$x = $x + $d
			return $x
		}

		print(strval($adder(5)))
		print(strval($adder(15)))
		print(strval($step(3)))
		print(strval($step(7)))
		print("LOCAL_REASSIGN:" . strval($reassign(4)) . ":" . strval($x))
		print("MAP_TOTAL:" . strval($counters["hits"]) . ":" . strval($box->val))
	}
}
`
		srcPath := filepath.Join(workDir, "closures.joss")
		if err := os.WriteFile(srcPath, []byte(src), 0644); err != nil {
			t.Fatalf("write closures.joss: %v", err)
		}

		outName := "closures_auto"
		if runtime.GOOS == "windows" {
			outName += ".exe"
		}
		outPath := filepath.Join(workDir, outName)

		buildCmd := exec.Command(jossExe, "build", srcPath, "-o", outPath)
		buildCmd.Dir = workDir
		buildOut, err := buildCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("joss build (auto backend) failed: %v\noutput:\n%s", err, string(buildOut))
		}
		if strings.Contains(string(buildOut), "compatibilidad Go") {
			t.Fatalf("expected auto backend to compile closures natively without Go compat fallback:\n%s", string(buildOut))
		}
		assertPureNativeBinaryWithoutGoOrTrailer(t, outPath)

		nativeOutBytes, err := exec.Command(outPath).CombinedOutput()
		if err != nil {
			t.Fatalf("native closure binary failed: %v\noutput:\n%s", err, string(nativeOutBytes))
		}

		antCmd := exec.Command(jossExe, "run", "--anterior", srcPath)
		antCmd.Dir = workDir
		antOutBytes, err := antCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("joss run --anterior failed: %v\noutput:\n%s", err, string(antOutBytes))
		}

		norm := func(b []byte, stripSecurityBanner bool) string {
			text := strings.ReplaceAll(string(b), "\r\n", "\n")
			lines := strings.Split(text, "\n")
			filtered := make([]string, 0, len(lines))
			for _, line := range lines {
				if stripSecurityBanner && (strings.HasPrefix(line, "[Security] ") || strings.HasPrefix(line, "[AssetManager] ")) {
					continue
				}
				filtered = append(filtered, line)
			}
			return strings.TrimSpace(strings.Join(filtered, "\n"))
		}
		nativeOut := norm(nativeOutBytes, false)
		antOut := norm(antOutBytes, true)
		if nativeOut != antOut {
			t.Fatalf("auto-backend closure output diverges from --anterior\nnative:\n%s\n--anterior:\n%s", nativeOut, antOut)
		}
		for _, wantLine := range []string{
			"SUM:105",
			"SUM:115",
			"ACC:8:1:108",
			"ACC:15:2:115",
			"LOCAL_REASSIGN:14:10",
			"MAP_TOTAL:2:15",
		} {
			if !strings.Contains(nativeOut, wantLine) {
				t.Fatalf("missing expected line %q in closure output:\n%s", wantLine, nativeOut)
			}
		}
	})

	t.Run("schema_ddl_c11_autowire_without_grandb_query", func(t *testing.T) {
		// Compila con el flujo C11 nativo sin ninguna llamada previa a GranDB::query
		// verificando joss_db_auto_register_schema en joss_db_rt.c sobre PG wire en cualquier OS.
		schemaSrc := `
public class Main {
	Init main() {
		bool $beforeItems = Schema::hasTable("items")
		Schema::create("items", func(Blueprint $table): void {
			$table->id("id")
			$table->string("title", 120)
		})
		bool $afterCreate = Schema::hasTable("items")
		bool $hasTitle = Schema::hasColumn("items", "title")
		bool $hasQtyPre = Schema::hasColumn("items", "qty")

		Schema::table("items", func(Blueprint $table): void {
			$table->integer("qty")
		})
		bool $hasQtyPost = Schema::hasColumn("items", "qty")

		Schema::rename("items", "inventory")
		bool $oldExists = Schema::hasTable("items")
		bool $newExists = Schema::hasTable("inventory")
		bool $newHasQty = Schema::hasColumn("inventory", "qty")

		Schema::dropIfExists("inventory")
		bool $afterDrop = Schema::hasTable("inventory")

		print("SCHEMA_DDL:" . strval($beforeItems) . "|" . strval($afterCreate) . "|" . strval($hasTitle) . "|" . strval($hasQtyPre) . "|" . strval($hasQtyPost) . "|" . strval($oldExists) . "|" . strval($newExists) . "|" . strval($newHasQty) . "|" . strval($afterDrop))
	}
}
`
		schemaExe := buildNativeTestServerExe(t, schemaSrc, "schema_ddl_c11")
		assertPureNativeBinaryWithoutGoOrTrailer(t, schemaExe)

		port, sqlLog, mu := startMockPGSchemaServer(t, "pguser", "pgpass123")
		cmd := exec.Command(schemaExe)
		cmd.Env = append(os.Environ(),
			"DB_CONNECTION=pgsql",
			"DB_HOST=127.0.0.1",
			fmt.Sprintf("DB_PORT=%d", port),
			"DB_USERNAME=pguser",
			"DB_PASSWORD=pgpass123",
			"DB_DATABASE=appdb",
		)
		outBytes, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("schema_ddl_c11 execution failed: %v\noutput:\n%s", err, string(outBytes))
		}
		got := strings.TrimSpace(string(outBytes))
		want := "SCHEMA_DDL:false|true|true|false|true|false|true|true|false"
		if !strings.Contains(got, want) {
			t.Fatalf("unexpected Schema DDL output\nwant contains: %s\ngot:\n%s", want, got)
		}

		mu.Lock()
		logged := append([]string(nil), (*sqlLog)...)
		mu.Unlock()
		joinedSQL := strings.Join(logged, "\n")
		for _, wantSQL := range []string{
			"CREATE TABLE IF NOT EXISTS items",
			"ALTER TABLE items ADD COLUMN qty",
			"ALTER TABLE items RENAME TO inventory",
			"DROP TABLE IF EXISTS inventory",
		} {
			if !strings.Contains(joinedSQL, wantSQL) {
				t.Fatalf("missing wire SQL %q; captured queries:\n%s", wantSQL, joinedSQL)
			}
		}
	})
}
