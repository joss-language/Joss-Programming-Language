package core

import (
	"path/filepath"
	"testing"
	"time"
)

func TestNativeStreamIO(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "stream_test.txt")
	escapedPath := filepath.ToSlash(testFile)

	code := `
$writer = new StreamWriter();
$writer->open("` + escapedPath + `", false);
$writer->writeLine("Hola mundo desde Joss Stream");
$writer->writeLine("Segunda linea de prueba");
$writer->close();

$reader = new StreamReader();
$reader->open("` + escapedPath + `");
$l1 = $reader->readLine();
$l2 = $reader->readLine();
$reader->close();

$fs = new FileStream();
$fs->open("` + escapedPath + `", "r");
$sz = $fs->size();
$fs->close();
`
	r := executeCode(t, code)
	if r.Variables["l1"] != "Hola mundo desde Joss Stream" {
		t.Fatalf("expected l1='Hola mundo desde Joss Stream', got %v", r.Variables["l1"])
	}
	if r.Variables["l2"] != "Segunda linea de prueba" {
		t.Fatalf("expected l2='Segunda linea de prueba', got %v", r.Variables["l2"])
	}
	sz, ok := r.Variables["sz"].(int64)
	if !ok || sz <= 0 {
		t.Fatalf("expected positive file size, got %v", r.Variables["sz"])
	}
}

func TestNativeDateTime(t *testing.T) {
	code := `
$dt = DateTime::create(2026, 9, 21, 15, 30, 0);
$formatted = $dt->format("Y-m-d H:i:s");
$year = $dt->year();
$month = $dt->month();
$day = $dt->day();
$ts = $dt->timestamp();

$twoDays = DateInterval::days(2);
$future = $dt->add($twoDays);
$futureDay = $future->day();

$diff = $future->diff($dt);
$diffSecs = $diff->totalSeconds();
`
	r := executeCode(t, code)
	if r.Variables["formatted"] != "2026-09-21 15:30:00" {
		t.Fatalf("expected formatted='2026-09-21 15:30:00', got %v", r.Variables["formatted"])
	}
	if r.Variables["year"] != int64(2026) {
		t.Fatalf("expected year=2026, got %v", r.Variables["year"])
	}
	if r.Variables["month"] != int64(9) {
		t.Fatalf("expected month=9, got %v", r.Variables["month"])
	}
	if r.Variables["day"] != int64(21) {
		t.Fatalf("expected day=21, got %v", r.Variables["day"])
	}
	if r.Variables["futureDay"] != int64(23) {
		t.Fatalf("expected futureDay=23, got %v", r.Variables["futureDay"])
	}
	if r.Variables["diffSecs"] != int64(2*24*3600) {
		t.Fatalf("expected diffSecs=%d, got %v", 2*24*3600, r.Variables["diffSecs"])
	}
}

func TestNativeSyncPrimitives(t *testing.T) {
	code := `
$mu = new Mutex();
$mu->lock();
$locked = $mu->tryLock(); // should fail because already locked
$mu->unlock();
$unlocked = $mu->tryLock(); // should succeed
$mu->unlock();

$wg = new WaitGroup();
$wg->add(1);
$wg->done();
$wg->wait(); // should return immediately without deadlock
$ok = true;
`
	r := executeCode(t, code)
	if r.Variables["locked"] != false {
		t.Fatalf("expected tryLock while locked to be false, got %v", r.Variables["locked"])
	}
	if r.Variables["unlocked"] != true {
		t.Fatalf("expected tryLock when unlocked to be true, got %v", r.Variables["unlocked"])
	}
	if r.Variables["ok"] != true {
		t.Fatalf("expected wg to finish successfully")
	}
}

func TestNativeSocket(t *testing.T) {
	code := `
$server = new Socket();
$server->listen("127.0.0.1", "0");
$port = $server->port();
`
	r := executeCode(t, code)
	port, ok := r.Variables["port"].(int64)
	if !ok || port <= 0 {
		t.Fatalf("expected valid server port > 0, got %v", r.Variables["port"])
	}

	// Client connection and communication
	clientCode := `
$server = new Socket();
$server->listen("127.0.0.1", "0");
$port = $server->port();

$task = async {
    $client = new Socket();
    $client->connect("127.0.0.1", strval($port));
    $client->send("PING");
    $client->close();
    return true;
};

$conn = $server->accept();
$msg = $conn->receive(10);
$conn->close();
$server->close();
await $task;
`
	r2 := executeCode(t, clientCode)
	time.Sleep(50 * time.Millisecond)
	if r2.Variables["msg"] != "PING" {
		t.Fatalf("expected msg='PING', got %v", r2.Variables["msg"])
	}
}
