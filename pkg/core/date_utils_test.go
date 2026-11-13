package core

import (
	"testing"
	"time"
)

func TestFormatDateDefaultTokensAndEscapes(t *testing.T) {
	local := time.Date(2026, time.November, 12, 9, 7, 5, 0, time.Local)
	if got, want := FormatDate("", local), "2026-11-12 09:07:05"; got != want {
		t.Fatalf(`FormatDate("") = %q, want %q`, got, want)
	}
	if got, want := FormatDate(`\Y-\m-\d H:i:s`, local), "Y-m-d 09:07:05"; got != want {
		t.Fatalf("escaped date format = %q, want %q", got, want)
	}
}

func TestStrtotimePreservesDatabaseDateTimeValue(t *testing.T) {
	want := time.Now().Add(time.Hour).Truncate(time.Second)
	runtime := NewRuntime()
	defer runtime.Free()
	got, handled := runtime.callBuiltinDate("strtotime", []interface{}{want})
	if !handled {
		t.Fatal("strtotime was not handled")
	}
	if got != want.Unix() {
		t.Fatalf("strtotime(time.Time) = %#v, want Unix timestamp %d", got, want.Unix())
	}
}

func TestParseHumanTimeKeepsRFC3339UTCAndLocalDatetimeSemantics(t *testing.T) {
	base := time.Now()
	utcInput := base.UTC().Add(time.Hour).Format(time.RFC3339)
	parsedUTC, err := ParseHumanTime(utcInput, base)
	if err != nil {
		t.Fatalf("ParseHumanTime(RFC3339) failed: %v", err)
	}
	wantUTC, err := time.Parse(time.RFC3339, utcInput)
	if err != nil {
		t.Fatal(err)
	}
	if !parsedUTC.Equal(wantUTC) {
		t.Fatalf("ParseHumanTime(%q) = %s, want %s", utcInput, parsedUTC, wantUTC)
	}

	localInput := base.Add(time.Hour).Format("2006-01-02 15:04:05")
	parsedLocal, err := ParseHumanTime(localInput, base)
	if err != nil {
		t.Fatalf("ParseHumanTime(local DATETIME) failed: %v", err)
	}
	if delta := parsedLocal.Sub(base.Add(time.Hour).Truncate(time.Second)); delta < -time.Second || delta > time.Second {
		t.Fatalf("ParseHumanTime(local DATETIME) differs by %s", delta)
	}
}
