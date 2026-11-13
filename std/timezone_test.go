package std_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	semanticanalyzer "github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/std"
)

func TestPureJossTZifOffsetParser(t *testing.T) {
	// TZif v1: offset +01:00 before the first transition, +02:00 between
	// transitions, then back to +01:00.
	header := make([]byte, 44)
	copy(header, "TZif\x00")
	binary.BigEndian.PutUint32(header[32:36], 2)
	binary.BigEndian.PutUint32(header[36:40], 2)
	binary.BigEndian.PutUint32(header[40:44], 8)
	zif := append(header,
		0, 1, 134, 160, // transition 100000
		0, 3, 13, 64, // transition 200000
		1, 0, // transition type indexes
		0, 0, 14, 16, 0, 0, // +01:00 standard
		0, 0, 28, 32, 1, 4, // +02:00 daylight
		'S', 'T', 'D', 0, 'D', 'S', 'T', 0,
	)
	byteLiterals := make([]string, len(zif))
	for index, value := range zif {
		byteLiterals[index] = fmt.Sprint(value)
	}
	fixture := fmt.Sprintf(`
array<int> $tzif = [%s]
print(TimeZoneRules::offsetSeconds($tzif, 99999));
print(TimeZoneRules::offsetSeconds($tzif, 150000));
print(TimeZoneRules::offsetSeconds($tzif, 200000));
array<int> $bad = [];
print(TimeZoneRules::offsetSeconds($bad, 150000));
print(TimeZoneRules::abbreviation($tzif, 99999));
print(TimeZoneRules::abbreviation($tzif, 150000));
print(DateFormatter::formatEpoch("Y-m-d H:i:s D l F M c O P T", 0, 3600, "STD"));
print(DateFormatter::formatEpoch("Y-m-d H:i:s", -1, 0));
print(DateFormatter::formatEpoch("", 0, 0));
print(DateFormatter::formatEpoch("\\Y", 0, 0));
print(strlen(now("Y-m-d H:i:s")));
`, strings.Join(byteLiterals, ","))
	programParser := parser.NewParser(parser.NewLexer(fixture))
	program := programParser.ParseProgram()
	if errors := programParser.Errors(); len(errors) != 0 {
		t.Fatalf("timezone test source parse failed: %v", errors)
	}
	units := append([]semanticanalyzer.SourceUnit{{Path: "timezone_test.joss", Program: program}}, std.GetStdlibUnits()...)
	analysis := core.AnalyzeSourceUnits(units)
	if analysis.HasErrors() {
		t.Fatalf("timezone test source analysis failed: %v", analysis.Diagnostics)
	}
	var output bytes.Buffer
	runtime := core.NewRuntime()
	runtime.Env = map[string]string{"TZ": "UTC"}
	runtime.Out = &output
	defer runtime.Free()
	runtime.ExecutePrepared(analysis.Prepared)
	if got, want := output.String(), "3600\n7200\n3600\n0\nSTD\nDST\n1970-01-01 01:00:00 Thu Thursday January Jan 1970-01-01T01:00:00+01:00 +0100 +01:00 STD\n1969-12-31 23:59:59\n1970-01-01 00:00:00\nY\n19\n"; got != want {
		t.Fatalf("TZif offset parser output = %q, want %q", got, want)
	}
}
