package analyzer

import (
	"testing"

	"github.com/jossecurity/joss/pkg/typesystem"
)

func testChannelEnv() Environment {
	env := NewEnvironment()
	env.Builtins["send"] = Callable{
		Name:       "send",
		Parameters: []Parameter{{Name: "channel", Type: typesystem.Type{Kind: typesystem.Channel}}, {Name: "value", Type: typesystem.Type{Kind: typesystem.Mixed}}},
		ReturnType: typesystem.Type{Kind: typesystem.Bool},
	}
	env.Builtins["recv"] = Callable{
		Name:       "recv",
		Parameters: []Parameter{{Name: "channel", Type: typesystem.Type{Kind: typesystem.Channel}}},
		ReturnType: typesystem.Type{Kind: typesystem.Mixed},
	}
	return env
}

func TestChannelSendRejectsIncompatibleType(t *testing.T) {
	src := `
public func worker(channel<int> $ch) {
    send($ch, "hello")
}
`
	items := analyzeSource(t, src, testChannelEnv())
	if !hasCode(items, "JOSS-TYPE-003") {
		t.Fatalf("expected JOSS-TYPE-003 when sending string to channel<int>, got %#v", items)
	}
}

func TestChannelSendAcceptsCompatibleType(t *testing.T) {
	src := `
public func worker(channel<int> $ch) {
    send($ch, 42)
}
`
	items := analyzeSource(t, src, testChannelEnv())
	if hasCode(items, "JOSS-TYPE-003") {
		t.Fatalf("did not expect JOSS-TYPE-003 when sending int to channel<int>, got %#v", items)
	}
}

func TestChannelRecvRefinesType(t *testing.T) {
	src := `
public func reader(channel<string> $ch): int {
    string $val = recv($ch)
    return 1
}
`
	items := analyzeSource(t, src, testChannelEnv())
	if hasCode(items, "JOSS-TYPE-002") {
		t.Fatalf("did not expect JOSS-TYPE-002 for recv from channel<string>, got %#v", items)
	}
}
