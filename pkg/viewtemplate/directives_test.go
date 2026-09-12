package viewtemplate

import (
	"strings"
	"testing"
)

func TestScanRecognizesCurrentDirectiveSurface(t *testing.T) {
	source := `@extends('layout') @section("body") @json(build({"x": call(1)})) @endsection @yield('body') @include('nav') @foreach($xs as $x) @endforeach`
	directives, err := Scan(source)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"extends", "section", "json", "endsection", "yield", "include", "foreach", "endforeach"}
	if len(directives) != len(want) {
		t.Fatalf("directives = %#v", directives)
	}
	for index, name := range want {
		if directives[index].Name != name {
			t.Fatalf("directive %d = %q, want %q", index, directives[index].Name, name)
		}
	}
	if directives[2].Arguments != `build({"x": call(1)})` {
		t.Fatalf("nested @json argument = %q", directives[2].Arguments)
	}
}

func TestRewriteJSONHandlesNestedExpressionsAndQuotes(t *testing.T) {
	result, err := RewriteJSON(`<p>@json(build("value)", nested(1)))</p>`)
	if err != nil {
		t.Fatal(err)
	}
	if result != `<p>{{! json_encode(build("value)", nested(1))) }}</p>` {
		t.Fatalf("rewrite = %q", result)
	}
}

func TestScanRejectsUnterminatedDirective(t *testing.T) {
	if _, err := Scan(`@json(call(1)`); err == nil {
		t.Fatal("unterminated directive was accepted")
	}
}

func FuzzDirectiveScanner(f *testing.F) {
	for _, seed := range []string{"@json($x)", "@json(call(1, nested(2)))", "@section('x')", "@foreach($xs as $x)", "plain"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, source string) {
		first, firstErr := Scan(source)
		second, secondErr := Scan(source)
		if (firstErr == nil) != (secondErr == nil) {
			t.Fatal("parse is not deterministic")
		}
		if firstErr == nil {
			if len(first) != len(second) {
				t.Fatal("parse length is not deterministic")
			}
			for _, directive := range first {
				if directive.Start < 0 || directive.End < directive.Start || directive.End > len(source) {
					t.Fatalf("invalid range: %#v", directive)
				}
			}
		}
		_, _ = RewriteJSON(strings.Clone(source))
	})
}
