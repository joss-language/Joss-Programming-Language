package typesystem

import (
	"testing"
)

func TestParseChannelGenerics(t *testing.T) {
	chInt := Parse("channel<int>")
	if chInt.Kind != Channel || chInt.Element == nil || chInt.Element.Kind != Int {
		t.Fatalf("expected Channel of Int, got %#v", chInt)
	}
	if chInt.String() != "channel<int>" {
		t.Fatalf("expected string representation channel<int>, got %s", chInt.String())
	}

	chStr := Parse("channel<string>")
	if chStr.Kind != Channel || chStr.Element == nil || chStr.Element.Kind != String {
		t.Fatalf("expected Channel of String, got %#v", chStr)
	}

	// Test assignability: channel<int> is not assignable to channel<string>
	if Assignable(chStr, chInt) {
		t.Fatal("channel<int> should not be assignable to channel<string>")
	}
	if !Assignable(chInt, chInt) {
		t.Fatal("channel<int> should be assignable to channel<int>")
	}
}

func TestParseResultGenerics(t *testing.T) {
	resType := Parse("Result<int, string>")
	if resType.Kind != Class || resType.Name != "Result" || resType.Key == nil || resType.Element == nil {
		t.Fatalf("expected Result<int, string>, got %#v", resType)
	}
	if resType.Key.Kind != Int || resType.Element.Kind != String {
		t.Fatalf("expected Key=Int and Element=String, got Key=%#v, Element=%#v", resType.Key, resType.Element)
	}
	if resType.String() != "Result<int, string>" {
		t.Fatalf("expected string Result<int, string>, got %s", resType.String())
	}

	resOther := Parse("Result<float, string>")
	if Assignable(resType, resOther) {
		t.Fatal("Result<float, string> should not be assignable to Result<int, string>")
	}
	if !Assignable(resType, resType) {
		t.Fatal("Result<int, string> should be assignable to itself")
	}
}
