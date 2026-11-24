package native_test

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeParity_Phase1_DateTimeAndDateInterval(t *testing.T) {
	src := `
var $dt = DateTime::create(2026, 3, 15);
print("FMT1:" . $dt->format("Y-m-d H:i:s"));
print("FMT2:" . $dt->format("y/m/d h:i:s a A"));
print("PARTS:" . strval($dt->year()) . "-" . strval($dt->month()) . "-" . strval($dt->day()) . " " . strval($dt->hour()) . ":" . strval($dt->minute()) . ":" . strval($dt->second()));
print("ISO:" . $dt->iso());

var $parsed = DateTime::parse("2026-07-04 14:35:50");
print("PARSED:" . $parsed->format("Y-m-d H:i:s"));
print("P_PARTS:" . strval($parsed->year()) . "," . strval($parsed->month()) . "," . strval($parsed->day()) . "," . strval($parsed->hour()) . "," . strval($parsed->minute()) . "," . strval($parsed->second()));

var $d5 = DateInterval::days(5);
var $h3 = DateInterval::hours(3);
var $m15 = DateInterval::minutes(15);
var $s40 = DateInterval::seconds(40);
print("DI:" . strval($d5->totalSeconds()) . "," . strval($h3->totalSeconds()) . "," . strval($m15->totalSeconds()) . "," . strval($s40->totalSeconds()));

var $added = $parsed->add($d5)->add($h3)->add($m15)->add($s40);
print("ADDED:" . $added->format("Y-m-d H:i:s"));
var $subbed = $added->sub($d5);
print("SUBBED:" . $subbed->format("Y-m-d H:i:s"));
var $diff = $added->diff($parsed);
print("DIFF:" . strval($diff->totalSeconds()));
var $diffRev = $parsed->diff($added);
print("DIFF_REV:" . strval($diffRev->totalSeconds()));

var $nowDt = DateTime::now();
print("NOW_POS:" . strval($nowDt->timestamp() > 1700000000));
`
	wbRunCLISourceAcrossBackends(t, "datetime_interval_parity", src, []string{"llvm"}, nil)
}

func TestNativeParity_Phase2_StreamsAndFileStream(t *testing.T) {
	dir := t.TempDir()
	p1 := filepath.ToSlash(filepath.Join(dir, "lines.txt"))
	p2 := filepath.ToSlash(filepath.Join(dir, "fs.txt"))
	src := fmt.Sprintf(`
string $p1 = %q;
string $p2 = %q;

// 1. StreamWriter write + writeLine + append
var $w = StreamWriter::open($p1, false);
int $n1 = $w->writeLine("alpha");
int $n2 = $w->writeLine("beta");
$w->flush();
$w->close();

var $wa = StreamWriter::open($p1, true);
int $n3 = $wa->write("gamma");
$wa->flush();
$wa->close();
print("WRITTEN:" . strval($n1) . "," . strval($n2) . "," . strval($n3));

// 2. StreamReader readLine until null EOF
var $r = StreamReader::open($p1);
int $idx = 0;
while (true) {
	string|null $line = $r->readLine();
	($line == null) ? {
		break;
	}
	print("L" . strval($idx) . ":" . strval($line));
	$idx++;
}
$r->close();

// 3. StreamReader readToEnd
var $r2 = StreamReader::open($p1);
string $all = $r2->readToEnd();
$r2->close();
print("ALL_LEN:" . strval(strlen($all)));

// 4. FileStream write, size, seek, read, flush, close + StreamReader over FileStream
var $fw = FileStream::open($p2, "w");
int $wn = $fw->write("0123456789ABCDEF");
$fw->flush();
int $sz = $fw->size();
$fw->close();
print("FS_W:" . strval($wn) . "|SZ:" . strval($sz));

var $fr = FileStream::open($p2, "r");
int $pos = $fr->seek(6, 0);
string $slice = $fr->read(4);
print("FS_SEEK:" . strval($pos) . "|SLICE:" . $slice);
int $pos2 = $fr->seek(0, 0);
var $sr = StreamReader::open($fr);
print("FS_SR:" . strval($pos2) . "|" . $sr->readToEnd());
$sr->close();
$fr->close();
`, p1, p2)
	wbRunCLISourceAcrossBackends(t, "streams_parity", src, []string{"llvm"}, nil)
}

func TestNativeParity_Phase3_ZipExtract(t *testing.T) {
	dir := t.TempDir()
	goodZip := filepath.Join(dir, "good.zip")
	slipZip := filepath.Join(dir, "slip.zip")
	destGood := filepath.Join(dir, "out_good")
	destSlip := filepath.Join(dir, "out_slip")

	// Build good.zip with STORE + DEFLATE + nested dir
	deflatedContent := strings.Repeat("Joss Native DEFLATE parity test line 0123456789!\n", 40)
	{
		f, err := os.Create(goodZip)
		if err != nil {
			t.Fatalf("create good.zip: %v", err)
		}
		zw := zip.NewWriter(f)
		// 1. STORE entry
		h1 := &zip.FileHeader{Name: "stored.txt", Method: zip.Store}
		w1, err := zw.CreateHeader(h1)
		if err != nil {
			t.Fatalf("create stored.txt: %v", err)
		}
		_, _ = w1.Write([]byte("hello stored zip"))

		// 2. Directory entry
		_, _ = zw.Create("empty_dir/")

		// 3. DEFLATE entry in nested dir
		h2 := &zip.FileHeader{Name: "nested/sub/deflated.txt", Method: zip.Deflate}
		w2, err := zw.CreateHeader(h2)
		if err != nil {
			t.Fatalf("create deflated.txt: %v", err)
		}
		_, _ = w2.Write([]byte(deflatedContent))

		if err := zw.Close(); err != nil {
			t.Fatalf("close good.zip: %v", err)
		}
		_ = f.Close()
	}

	// Build slip.zip with Zip-Slip entry
	{
		f, err := os.Create(slipZip)
		if err != nil {
			t.Fatalf("create slip.zip: %v", err)
		}
		zw := zip.NewWriter(f)
		w, err := zw.Create("../escape.txt")
		if err != nil {
			t.Fatalf("create slip entry: %v", err)
		}
		_, _ = w.Write([]byte("evil"))
		_ = zw.Close()
		_ = f.Close()
	}

	src := fmt.Sprintf(`
string $goodZip = %q;
string $destGood = %q;
string $slipZip = %q;
string $destSlip = %q;

bool $ok1 = Zip::extract($goodZip, $destGood);
print("OK1:" . strval($ok1));

var $sr1 = StreamReader::open($destGood . "/stored.txt");
print("STORED:" . $sr1->readToEnd());
$sr1->close();

var $sr2 = StreamReader::open($destGood . "/nested/sub/deflated.txt");
string $def = $sr2->readToEnd();
$sr2->close();
print("DEFLATED_LEN:" . strval(strlen($def)));
print("DEFLATED_HEAD:" . substr($def, 0, 24));

bool $ok2 = Zip::extract($slipZip, $destSlip);
print("OK2:" . strval($ok2));
`, filepath.ToSlash(goodZip), filepath.ToSlash(destGood), filepath.ToSlash(slipZip), filepath.ToSlash(destSlip))

	wbRunCLISourceAcrossBackends(t, "zip_extract_parity", src, []string{"llvm"}, nil)

	// Ensure Zip-Slip did NOT write escape.txt
	if _, err := os.Stat(filepath.Join(dir, "escape.txt")); err == nil {
		t.Fatalf("Zip-Slip wrote escape.txt outside destination!")
	}
}
