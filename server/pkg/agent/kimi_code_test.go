package agent

import (
	"log/slog"
	"strings"
	"testing"
)

func TestKimiCodeProcessAssistantText(t *testing.T) {
	t.Parallel()

	b := &kimiCodeBackend{cfg: Config{}}
	ch := make(chan Message, 256)

	line := `{"role":"assistant","content":[{"type":"text","text":"Hello, world!"}]}`

	result := b.processEvents(strings.NewReader(line), ch)

	if result.status != "completed" {
		t.Fatalf("expected completed, got %q", result.status)
	}
	if result.output != "Hello, world!" {
		t.Fatalf("expected output 'Hello, world!', got %q", result.output)
	}

	var msgs []Message
	for len(msgs) < 2 {
		select {
		case m := <-ch:
			msgs = append(msgs, m)
		default:
			t.Fatal("expected 2 messages, got", len(msgs))
		}
	}
	if msgs[0].Type != MessageStatus || msgs[0].Status != "running" {
		t.Fatalf("expected first message to be status running, got %+v", msgs[0])
	}
	if msgs[1].Type != MessageText || msgs[1].Content != "Hello, world!" {
		t.Fatalf("expected second message to be text, got %+v", msgs[1])
	}
}

func TestKimiCodeProcessAssistantThinking(t *testing.T) {
	t.Parallel()

	b := &kimiCodeBackend{cfg: Config{}}
	ch := make(chan Message, 256)

	line := `{"role":"assistant","content":[{"type":"think","think":"I need to check the files"},{"type":"text","text":"Let me look at the files."}]}`

	result := b.processEvents(strings.NewReader(line), ch)

	if result.output != "Let me look at the files." {
		t.Fatalf("expected text output, got %q", result.output)
	}

	var msgs []Message
	for len(msgs) < 3 {
		select {
		case m := <-ch:
			msgs = append(msgs, m)
		default:
			t.Fatal("expected 3 messages")
		}
	}

	if msgs[1].Type != MessageThinking || msgs[1].Content != "I need to check the files" {
		t.Fatalf("expected thinking message, got %+v", msgs[1])
	}
	if msgs[2].Type != MessageText || msgs[2].Content != "Let me look at the files." {
		t.Fatalf("expected text message, got %+v", msgs[2])
	}
}

func TestKimiCodeProcessToolCalls(t *testing.T) {
	t.Parallel()

	b := &kimiCodeBackend{cfg: Config{}}
	ch := make(chan Message, 256)

	line := `{"role":"assistant","content":[{"type":"think","think":"Running ls"},{"type":"text","text":"Let me list the files."}],"tool_calls":[{"type":"function","id":"tool_abc123","function":{"name":"Shell","arguments":"{\"command\": \"ls -la\"}"}}]}`

	result := b.processEvents(strings.NewReader(line), ch)

	if result.output != "Let me list the files." {
		t.Fatalf("expected text output, got %q", result.output)
	}

	var msgs []Message
drain:
	for {
		select {
		case m := <-ch:
			msgs = append(msgs, m)
		default:
			break drain
		}
	}

	var toolMsg *Message
	for i := range msgs {
		if msgs[i].Type == MessageToolUse {
			toolMsg = &msgs[i]
			break
		}
	}
	if toolMsg == nil {
		t.Fatal("expected a tool_use message")
	}
	if toolMsg.Tool != "Shell" {
		t.Fatalf("expected tool 'Shell', got %q", toolMsg.Tool)
	}
	if toolMsg.CallID != "tool_abc123" {
		t.Fatalf("expected callID 'tool_abc123', got %q", toolMsg.CallID)
	}
	if toolMsg.Input["command"] != "ls -la" {
		t.Fatalf("expected input command 'ls -la', got %v", toolMsg.Input["command"])
	}
}

func TestKimiCodeProcessToolResult(t *testing.T) {
	t.Parallel()

	b := &kimiCodeBackend{cfg: Config{}}
	ch := make(chan Message, 256)

	line := `{"role":"tool","content":[{"type":"text","text":"<system>Command executed successfully.</system>"},{"type":"text","text":"total 42\ndrwxr-xr-x 5 user staff 160 Apr 21 ."}],"tool_call_id":"tool_abc123"}`

	b.processEvents(strings.NewReader(line), ch)

	var msgs []Message
drain:
	for {
		select {
		case m := <-ch:
			msgs = append(msgs, m)
		default:
			break drain
		}
	}

	var resultMsg *Message
	for i := range msgs {
		if msgs[i].Type == MessageToolResult {
			resultMsg = &msgs[i]
			break
		}
	}
	if resultMsg == nil {
		t.Fatal("expected a tool_result message")
	}
	if resultMsg.CallID != "tool_abc123" {
		t.Fatalf("expected callID 'tool_abc123', got %q", resultMsg.CallID)
	}
	if !strings.Contains(resultMsg.Output, "total 42") {
		t.Fatalf("expected output to contain 'total 42', got %q", resultMsg.Output)
	}
}

func TestKimiCodeProcessMultipleToolResults(t *testing.T) {
	t.Parallel()

	b := &kimiCodeBackend{cfg: Config{}}
	ch := make(chan Message, 256)

	line := `{"role":"tool","content":[{"type":"text","text":"line1"},{"type":"text","text":"line2"}],"tool_call_id":"tool_xyz"}`

	b.processEvents(strings.NewReader(line), ch)

	var msgs []Message
drain:
	for {
		select {
		case m := <-ch:
			msgs = append(msgs, m)
		default:
			break drain
		}
	}

	var resultMsg *Message
	for i := range msgs {
		if msgs[i].Type == MessageToolResult {
			resultMsg = &msgs[i]
			break
		}
	}
	if resultMsg == nil {
		t.Fatal("expected a tool_result message")
	}
	if resultMsg.Output != "line1\nline2" {
		t.Fatalf("expected joined output 'line1\\nline2', got %q", resultMsg.Output)
	}
}

func TestKimiCodeProcessFullTurn(t *testing.T) {
	t.Parallel()

	b := &kimiCodeBackend{cfg: Config{}}
	ch := make(chan Message, 256)

	input := `{"role":"assistant","content":[{"type":"think","think":"I should check the files"},{"type":"text","text":"Let me list the files."}],"tool_calls":[{"type":"function","id":"tool_1","function":{"name":"Shell","arguments":"{\"command\": \"ls\"}"}}]}
{"role":"tool","content":[{"type":"text","text":"file1.txt\nfile2.txt"}],"tool_call_id":"tool_1"}
{"role":"assistant","content":[{"type":"text","text":"I found 2 files: file1.txt and file2.txt"}]}`

	result := b.processEvents(strings.NewReader(input), ch)

	if result.status != "completed" {
		t.Fatalf("expected completed, got %q", result.status)
	}
	expected := "Let me list the files.I found 2 files: file1.txt and file2.txt"
	if result.output != expected {
		t.Fatalf("expected output %q, got %q", expected, result.output)
	}
}

func TestKimiCodeProcessEmptyInput(t *testing.T) {
	t.Parallel()

	b := &kimiCodeBackend{cfg: Config{}}
	ch := make(chan Message, 256)

	result := b.processEvents(strings.NewReader(""), ch)

	if result.status != "completed" {
		t.Fatalf("expected completed, got %q", result.status)
	}
	if result.output != "" {
		t.Fatalf("expected empty output, got %q", result.output)
	}
}

func TestKimiCodeProcessInvalidJSON(t *testing.T) {
	t.Parallel()

	b := &kimiCodeBackend{cfg: Config{Logger: slog.Default()}}
	ch := make(chan Message, 256)

	input := "not json\nalso not json\n"

	result := b.processEvents(strings.NewReader(input), ch)

	if result.status != "completed" {
		t.Fatalf("expected completed, got %q", result.status)
	}
	if result.output != "" {
		t.Fatalf("expected empty output, got %q", result.output)
	}
}

func TestKimiCodeProcessEmptyLines(t *testing.T) {
	t.Parallel()

	b := &kimiCodeBackend{cfg: Config{}}
	ch := make(chan Message, 256)

	input := "\n\n{\"role\":\"assistant\",\"content\":[{\"type\":\"text\",\"text\":\"hi\"}]}\n\n"

	result := b.processEvents(strings.NewReader(input), ch)

	if result.output != "hi" {
		t.Fatalf("expected 'hi', got %q", result.output)
	}
}

func TestKimiCodeBlockedArgs(t *testing.T) {
	t.Parallel()

	expectedBlocked := []string{"-p", "--prompt", "--output-format", "-S", "--session", "-r"}
	for _, arg := range expectedBlocked {
		if _, ok := kimiCodeBlockedArgs[arg]; !ok {
			t.Errorf("expected %q to be in kimiCodeBlockedArgs", arg)
		}
	}
}

func TestKimiCodeBuildArgs(t *testing.T) {
	t.Parallel()

	args := buildKimiCodeArgs("test-session", "hello world", ExecOptions{Model: "kimi-for-coding"}, slog.Default())

	argStr := strings.Join(args, " ")
	if !strings.Contains(argStr, "-p hello world") {
		t.Error("expected -p <prompt> in args")
	}
	if !strings.Contains(argStr, "--output-format stream-json") {
		t.Error("expected --output-format stream-json in args")
	}
	if strings.Contains(argStr, "-y") {
		t.Error("-y should not be in args (prompt mode auto-approves)")
	}
	if !strings.Contains(argStr, "-S test-session") {
		t.Error("expected -S test-session in args")
	}
	if !strings.Contains(argStr, "-m kimi-for-coding") {
		t.Error("expected -m kimi-for-coding in args")
	}
}

func TestKimiCodeBuildArgsWithResumeSession(t *testing.T) {
	t.Parallel()

	args := buildKimiCodeArgs("prior-session-id", "resume this", ExecOptions{ResumeSessionID: "prior-session-id"}, slog.Default())

	argStr := strings.Join(args, " ")
	if !strings.Contains(argStr, "-S prior-session-id") {
		t.Error("expected -S prior-session-id in args")
	}
}

func TestKimiCodeBuildArgsFiltersBlocked(t *testing.T) {
	t.Parallel()

	args := buildKimiCodeArgs("s1", "prompt", ExecOptions{
		CustomArgs: []string{"--output-format", "text"},
	}, slog.Default())

	// The custom "text" value should be filtered (daemon uses "stream-json").
	// Count occurrences: daemon puts one "--output-format stream-json",
	// the custom "--output-format text" should be stripped entirely.
	outputCount := 0
	for _, arg := range args {
		if arg == "--output-format" {
			outputCount++
		}
	}
	if outputCount != 1 {
		t.Errorf("expected exactly 1 --output-format (daemon-managed), got %d", outputCount)
	}
}

func TestKimiCodeProcessSystemRole(t *testing.T) {
	t.Parallel()

	b := &kimiCodeBackend{cfg: Config{}}
	ch := make(chan Message, 256)

	line := `{"role":"system","content":[{"type":"text","text":"system info"}]}`

	result := b.processEvents(strings.NewReader(line), ch)

	if result.status != "completed" {
		t.Fatalf("expected completed, got %q", result.status)
	}
	if result.output != "" {
		t.Fatalf("expected empty output (system messages ignored), got %q", result.output)
	}
}

func TestNewReturnsKimiCodeBackend(t *testing.T) {
	t.Parallel()
	b, err := New("kimi-code", Config{ExecutablePath: "/nonexistent/kimi-code"})
	if err != nil {
		t.Fatalf("New(kimi-code) error: %v", err)
	}
	if _, ok := b.(*kimiCodeBackend); !ok {
		t.Fatalf("expected *kimiCodeBackend, got %T", b)
	}
}

func TestKimiCodeProcessStringContent(t *testing.T) {
	t.Parallel()

	b := &kimiCodeBackend{cfg: Config{}}
	ch := make(chan Message, 256)

	line := `{"role":"assistant","content":"Hello from Kimi Code!"}`

	result := b.processEvents(strings.NewReader(line), ch)
	if result.output != "Hello from Kimi Code!" {
		t.Fatalf("expected 'Hello from Kimi Code!', got %q", result.output)
	}
}

func TestKimiCodeProcessMetaSessionID(t *testing.T) {
	t.Parallel()

	b := &kimiCodeBackend{cfg: Config{}}
	ch := make(chan Message, 256)

	input := `{"role":"assistant","content":"Hi!"}
{"role":"meta","type":"session.resume_hint","session_id":"session_a8642125-f735-46d0-820b-2c862f21a08a","command":"kimi -r session_a8642125-f735-46d0-820b-2c862f21a08a","content":"To resume this session: kimi -r session_a8642125-f735-46d0-820b-2c862f21a08a"}`

	result := b.processEvents(strings.NewReader(input), ch)
	if result.output != "Hi!" {
		t.Fatalf("expected 'Hi!', got %q", result.output)
	}
	if result.sessionID != "session_a8642125-f735-46d0-820b-2c862f21a08a" {
		t.Fatalf("expected session ID 'session_a8642125-f735-46d0-820b-2c862f21a08a', got %q", result.sessionID)
	}
}

func TestKimiCodeProcessToolResultStringContent(t *testing.T) {
	t.Parallel()

	b := &kimiCodeBackend{cfg: Config{}}
	ch := make(chan Message, 256)

	line := `{"role":"tool","content":"file1.txt\nfile2.txt","tool_call_id":"tool_abc123"}`

	b.processEvents(strings.NewReader(line), ch)

	var msgs []Message
drain:
	for {
		select {
		case m := <-ch:
			msgs = append(msgs, m)
		default:
			break drain
		}
	}

	var resultMsg *Message
	for i := range msgs {
		if msgs[i].Type == MessageToolResult {
			resultMsg = &msgs[i]
			break
		}
	}
	if resultMsg == nil {
		t.Fatal("expected a tool_result message")
	}
	if resultMsg.Output != "file1.txt\nfile2.txt" {
		t.Fatalf("expected 'file1.txt\\nfile2.txt', got %q", resultMsg.Output)
	}
}
