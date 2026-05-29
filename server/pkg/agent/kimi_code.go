package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// kimiCodeBlockedArgs are flags hardcoded by the daemon that must not be
// overridden by user-configured custom_args.
var kimiCodeBlockedArgs = map[string]blockedArgMode{
	"-p":              blockedWithValue, // prompt is passed as -p argument
	"--prompt":        blockedWithValue, // prompt is passed as -p argument
	"--output-format": blockedWithValue, // stream-json protocol for daemon communication
	"-S":              blockedWithValue, // session ID managed by daemon
	"--session":       blockedWithValue, // session ID managed by daemon
	"-r":              blockedWithValue, // resume session managed by daemon
}

// kimiCodeBackend implements Backend by spawning the Kimi Code CLI
// with `-p <prompt> --output-format stream-json` and parsing its NDJSON event stream.
type kimiCodeBackend struct {
	cfg Config
}

// kimiCodeStderrWriter wraps a logWriter and captures the session ID from
// stderr lines like "To resume this session: kimi -r <sessionId>".
type kimiCodeStderrWriter struct {
	inner     *logWriter
	sessionID string
	buf       string
}

func (w *kimiCodeStderrWriter) Write(p []byte) (int, error) {
	w.buf += string(p)
	for {
		idx := strings.Index(w.buf, "\n")
		if idx < 0 {
			break
		}
		line := strings.TrimSpace(w.buf[:idx])
		w.buf = w.buf[idx+1:]
		if sIdx := strings.Index(line, "kimi -r "); sIdx >= 0 {
			id := strings.TrimSpace(line[sIdx+len("kimi -r "):])
			if id != "" {
				w.sessionID = id
			}
		}
	}
	return w.inner.Write(p)
}

func (b *kimiCodeBackend) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	execPath := b.cfg.ExecutablePath
	if execPath == "" {
		execPath = "kimi-code"
	}
	if _, err := exec.LookPath(execPath); err != nil {
		return nil, fmt.Errorf("kimi-code executable not found at %q: %w", execPath, err)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 20 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)

	// -S is for resuming an existing session only. On first invocation
	// kimi-code creates its own session; we omit -S entirely.
	args := buildKimiCodeArgs(opts.ResumeSessionID, prompt, opts, b.cfg.Logger)

	cmd := exec.CommandContext(runCtx, execPath, args...)
	hideAgentWindow(cmd)
	b.cfg.Logger.Debug("agent command", "exec", execPath, "args", args)
	cmd.WaitDelay = 10 * time.Second
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	cmd.Env = buildEnv(b.cfg.Env)

	// Prompt is passed via -p argument — no stdin pipe needed.

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("kimi-code stdout pipe: %w", err)
	}
	cmd.Stderr = &kimiCodeStderrWriter{inner: newLogWriter(b.cfg.Logger, "[kimi-code:stderr] ")}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start kimi-code: %w", err)
	}

	b.cfg.Logger.Info("kimi-code started", "pid", cmd.Process.Pid, "cwd", opts.Cwd, "model", opts.Model, "session", opts.ResumeSessionID)

	msgCh := make(chan Message, 256)
	resCh := make(chan Result, 1)

	// Close stdout when the context is cancelled so scanner.Scan() unblocks.
	go func() {
		<-runCtx.Done()
		_ = stdout.Close()
	}()

	go func() {
		defer cancel()
		defer close(msgCh)
		defer close(resCh)

		startTime := time.Now()
		scanResult := b.processEvents(stdout, msgCh)

		// Wait for process exit.
		exitErr := cmd.Wait()
		duration := time.Since(startTime)

		// Capture session ID from stderr.
		if sw, ok := cmd.Stderr.(*kimiCodeStderrWriter); ok && sw.sessionID != "" {
			scanResult.sessionID = sw.sessionID
		}

		if runCtx.Err() == context.DeadlineExceeded {
			scanResult.status = "timeout"
			scanResult.errMsg = fmt.Sprintf("kimi-code timed out after %s", timeout)
		} else if runCtx.Err() == context.Canceled {
			scanResult.status = "aborted"
			scanResult.errMsg = "execution cancelled"
		} else if exitErr != nil && scanResult.status == "completed" {
			scanResult.status = "failed"
			scanResult.errMsg = fmt.Sprintf("kimi-code exited with error: %v", exitErr)
		}

		b.cfg.Logger.Info("kimi-code finished", "pid", cmd.Process.Pid, "status", scanResult.status, "duration", duration.Round(time.Millisecond).String())

		// Build usage map. Kimi Code doesn't report token usage in stream-json
		// output, so we attribute any accumulated usage to the configured model.
		var usage map[string]TokenUsage
		u := scanResult.usage
		if u.InputTokens > 0 || u.OutputTokens > 0 || u.CacheReadTokens > 0 || u.CacheWriteTokens > 0 {
			model := opts.Model
			if model == "" {
				model = "unknown"
			}
			usage = map[string]TokenUsage{model: u}
		}

		resCh <- Result{
			Status:     scanResult.status,
			Output:     scanResult.output,
			Error:      scanResult.errMsg,
			DurationMs: duration.Milliseconds(),
			SessionID:  scanResult.sessionID,
			Usage:      usage,
		}
	}()

	return &Session{Messages: msgCh, Result: resCh}, nil
}

// ── Event handlers ──

// kimiCodeEventResult holds accumulated state from processing the event stream.
type kimiCodeEventResult struct {
	status    string
	errMsg    string
	output    string
	sessionID string
	usage     TokenUsage
}

// processEvents reads JSON lines from r, dispatches events to ch, and returns
// the accumulated result.
//
// Kimi Code's stream-json output emits one JSON object per message turn:
//
//   - {"role":"assistant","content":"...","tool_calls":[...]} — agent response
//   - {"role":"tool","content":"...","tool_call_id":"..."} — tool result
//   - {"role":"meta","type":"session.resume_hint","session_id":"..."} — session info
func (b *kimiCodeBackend) processEvents(r io.Reader, ch chan<- Message) kimiCodeEventResult {
	var output strings.Builder
	sessionID := ""
	finalStatus := "completed"
	var finalError string
	var usage TokenUsage
	emittedStatus := false

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var evt kimiCodeMessage
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			b.cfg.Logger.Warn("failed to unmarshal kimi-code event", "line", line, "error", err)
			continue
		}

		switch evt.Role {
		case "assistant":
			if !emittedStatus {
				trySend(ch, Message{Type: MessageStatus, Status: "running"})
				emittedStatus = true
			}

			if thinking := extractKimiCodeThinking(evt.Content); thinking != "" {
				trySend(ch, Message{Type: MessageThinking, Content: thinking})
			}

			text := extractKimiCodeText(evt.Content)
			if text != "" {
				output.WriteString(text)
				trySend(ch, Message{Type: MessageText, Content: text})
			}

			for _, tc := range evt.ToolCalls {
				var input map[string]any
				if tc.Function.Arguments != "" {
					_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
				}
				trySend(ch, Message{
					Type:   MessageToolUse,
					Tool:   tc.Function.Name,
					CallID: tc.ID,
					Input:  input,
				})
			}

		case "tool":
			toolOutput := extractKimiCodeText(evt.Content)
			trySend(ch, Message{
				Type:   MessageToolResult,
				CallID: evt.ToolCallID,
				Output: toolOutput,
			})

		case "meta":
			if evt.SessionID != "" {
				sessionID = evt.SessionID
			}

		case "system":
			// System messages are informational; skip.

		case "error":
			errMsg := extractKimiCodeText(evt.Content)
			if errMsg == "" {
				errMsg = "unknown kimi-code error"
			}
			trySend(ch, Message{Type: MessageError, Content: errMsg})
			finalStatus = "failed"
			finalError = errMsg
		}
	}

	if scanErr := scanner.Err(); scanErr != nil {
		b.cfg.Logger.Warn("kimi-code stdout scanner error", "error", scanErr)
		if finalStatus == "completed" {
			finalStatus = "failed"
			finalError = fmt.Sprintf("stdout read error: %v", scanErr)
		}
	}

	return kimiCodeEventResult{
		status:    finalStatus,
		errMsg:    finalError,
		output:    output.String(),
		sessionID: sessionID,
		usage:     usage,
	}
}

// ── JSON types for kimi-code -p --output-format stream-json ──

// kimiCodeMessage represents a single JSON message from Kimi Code's stream-json output.
type kimiCodeMessage struct {
	Role       string             `json:"role"`
	Content    json.RawMessage    `json:"content,omitempty"`
	ToolCalls  []kimiCodeToolCall `json:"tool_calls,omitempty"`
	ToolCallID string             `json:"tool_call_id,omitempty"`
	// Meta message fields (role == "meta")
	Type      string `json:"type,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// kimiCodeContentBlock represents a content block within a Kimi Code message.
type kimiCodeContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	// "think" type
	Think string `json:"think,omitempty"`
}

// kimiCodeToolCall represents a tool call within a Kimi Code assistant message.
type kimiCodeToolCall struct {
	Type     string               `json:"type"`
	ID       string               `json:"id"`
	Function kimiCodeFunctionCall `json:"function"`
}

// kimiCodeFunctionCall represents the function details of a tool call.
type kimiCodeFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string of arguments
}

// extractKimiCodeText extracts text from a raw JSON content field, which may
// be a plain string or an array of content blocks.
func extractKimiCodeText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var blocks []kimiCodeContentBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var parts []string
		for _, b := range blocks {
			if b.Text != "" {
				parts = append(parts, b.Text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

// extractKimiCodeThinking extracts thinking content from block-style content.
func extractKimiCodeThinking(raw json.RawMessage) string {
	var blocks []kimiCodeContentBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var parts []string
		for _, b := range blocks {
			if b.Type == "think" && b.Think != "" {
				parts = append(parts, b.Think)
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

// ── Arg builder ──

// buildKimiCodeArgs assembles the argv for a one-shot kimi-code invocation.
//
// Flags:
//
//	-p <prompt>                 non-interactive single prompt (auto-approves tools)
//	--output-format stream-json streaming NDJSON output for live events
//	-S <id>                     session ID for resumption
//	-m <model>                  optional model override
func buildKimiCodeArgs(sessionID string, prompt string, opts ExecOptions, logger *slog.Logger) []string {
	args := []string{
		"-p", prompt,
		"--output-format", "stream-json",
	}
	if sessionID != "" {
		args = append(args, "-S", sessionID)
	}
	if opts.Model != "" {
		args = append(args, "-m", opts.Model)
	}
	args = append(args, filterCustomArgs(opts.CustomArgs, kimiCodeBlockedArgs, logger)...)
	return args
}
