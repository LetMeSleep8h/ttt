package terminal

import (
	"strings"
	"testing"

	xterm "github.com/eugenioenko/xterm-go"
)

const fishPrompt = "\x1b]133;A;click_events=1\x1b\\/home/user/a/rather/long/project/path\r\n❯ \x1b]133;B\x1b\\"

func newPromptTerminal(cols, rows int) *Terminal {
	t := &Terminal{cols: cols, rows: rows}
	t.term = xterm.New(xterm.WithCols(cols), xterm.WithRows(rows), xterm.WithScrollback(100))
	t.watchPromptMarks()
	return t
}

// fish repaints after SIGWINCH by moving up to the first prompt row and
// clearing from there; a reflowed prompt used to survive that as a copy.
func TestResizeLeavesOnePromptAfterShellRedraw(t *testing.T) {
	term := newPromptTerminal(60, 10)
	term.term.WriteString("earlier output\r\n" + fishPrompt)

	for _, cols := range []int{20, 50, 15, 60} {
		term.resizeEmulator(cols, 10)
		term.term.WriteString("\r\x1b[A\x1b[J" + fishPrompt)
	}

	screen := term.term.String()
	if n := strings.Count(screen, "/home/user"); n != 1 {
		t.Fatalf("%d prompts after resizing, want 1:\n%s", n, screen)
	}
	if !strings.Contains(screen, "earlier output") {
		t.Fatalf("output above the prompt was lost:\n%s", screen)
	}
}

func TestResizeKeepsRunningCommandOutput(t *testing.T) {
	term := newPromptTerminal(60, 10)
	term.term.WriteString(fishPrompt + "ls\r\n\x1b]133;C\x1b\\file-one file-two")

	term.resizeEmulator(30, 10)

	if !strings.Contains(term.term.String(), "file-one") {
		t.Fatalf("command output cleared on resize:\n%s", term.term.String())
	}
}

// Layout passes resize terminals to the size they already have; no SIGWINCH
// follows, so clearing the prompt then would leave it blank.
func TestResizeToSameSizeKeepsPrompt(t *testing.T) {
	term := newPromptTerminal(60, 10)
	term.term.WriteString(fishPrompt)

	if term.resizeEmulator(60, 10) {
		t.Fatal("resize to the same size reported a change")
	}
	if !strings.Contains(term.term.String(), "/home/user") {
		t.Fatalf("prompt cleared by a same-size resize:\n%s", term.term.String())
	}
}

// A command whose output lacks a final newline leaves the next prompt on the
// same row; blanking the prompt must not take that output with it.
func TestResizeKeepsOutputBeforeSameRowPrompt(t *testing.T) {
	term := newPromptTerminal(60, 10)
	term.term.WriteString("partial" + fishPrompt)

	term.resizeEmulator(40, 10)

	screen := term.term.String()
	if !strings.Contains(screen, "partial") {
		t.Fatalf("output before the prompt was cleared:\n%s", screen)
	}
	if strings.Contains(screen, "/home/user") {
		t.Fatalf("prompt not cleared:\n%s", screen)
	}
}

const bashPrompt = "\x1b]133;A;redraw=last\a/home/user/project [main] ➜ "

// readlineRedraw is what bash sends after SIGWINCH: its cursor-up count comes
// from a prompt layout ttt cannot see, so the tests vary it.
func readlineRedraw(ups int, line string) string {
	return "\r\x1b[K" + strings.Repeat("\x1b[A", ups) + line
}

// feedOutput passes shell output to the emulator the way readLoop does.
func feedOutput(term *Terminal, s string) {
	out := []byte(s)
	if term.promptRedrawPending {
		out = term.takePromptRedraw(out)
	}
	term.term.Write(out)
}

func TestBashRedrawLeavesOnePrompt(t *testing.T) {
	term := newPromptTerminal(40, 10)
	feedOutput(term, "earlier output\r\n"+bashPrompt)

	for i, cols := range []int{25, 20, 15, 40} {
		term.resizeEmulator(cols, 10)
		feedOutput(term, readlineRedraw(i%3, bashPrompt))
	}

	screen := term.term.String()
	if n := strings.Count(screen, "/home/user"); n != 1 {
		t.Fatalf("%d prompts after resizing, want 1:\n%s", n, screen)
	}
	lines := strings.Split(screen, "\n")
	if lines[0] != "earlier output" || !strings.HasPrefix(lines[1], "/home/user") {
		t.Fatalf("prompt should follow the earlier output directly:\n%s", screen)
	}
}

func TestBashRedrawWithWrappedInput(t *testing.T) {
	term := newPromptTerminal(40, 10)
	input := "echo " + strings.Repeat("x", 30)
	feedOutput(term, "earlier output\r\n"+bashPrompt+input)

	term.resizeEmulator(20, 10)
	feedOutput(term, readlineRedraw(2, bashPrompt+input))

	screen := term.term.String()
	if n := strings.Count(screen, "/home/user"); n != 1 {
		t.Fatalf("%d prompts after resizing, want 1:\n%s", n, screen)
	}
	if !strings.HasPrefix(screen, "earlier output\n") {
		t.Fatalf("output above the prompt was overwritten:\n%s", screen)
	}
}

// readline repaints only the last line of a multi-line prompt, which is where
// the integration puts the prompt mark.
func TestBashRedrawKeepsEarlierPromptLines(t *testing.T) {
	term := newPromptTerminal(40, 10)
	feedOutput(term, "first prompt line\r\n"+bashPrompt)

	term.resizeEmulator(20, 10)
	feedOutput(term, readlineRedraw(1, bashPrompt))

	screen := term.term.String()
	if n := strings.Count(screen, "first prompt line"); n != 1 {
		t.Fatalf("first prompt line appears %d times, want 1:\n%s", n, screen)
	}
	if n := strings.Count(screen, "/home/user"); n != 1 {
		t.Fatalf("%d prompts after resizing, want 1:\n%s", n, screen)
	}
}

func TestBashRedrawIgnoresOtherOutput(t *testing.T) {
	term := newPromptTerminal(40, 10)
	feedOutput(term, bashPrompt)

	term.resizeEmulator(20, 10)
	feedOutput(term, "job done\r\n")
	if term.promptRedrawPending {
		t.Fatal("redraw still pending after unrelated output")
	}

	feedOutput(term, readlineRedraw(0, "kept"))
	if !strings.Contains(term.term.String(), "kept") {
		t.Fatalf("later output was rewritten:\n%s", term.term.String())
	}
}

func TestBashRedrawSkippedWhileCommandRuns(t *testing.T) {
	term := newPromptTerminal(40, 10)
	feedOutput(term, bashPrompt+"sleep 5\r\n\x1b]133;C\a")

	term.resizeEmulator(20, 10)

	if term.promptRedrawPending {
		t.Fatal("a running command's output would be treated as a prompt redraw")
	}
}

func TestBashRedrawWithDisposedMarker(t *testing.T) {
	term := newPromptTerminal(40, 10)
	feedOutput(term, bashPrompt)
	term.resizeEmulator(20, 10)
	term.promptMarker.Dispose()

	redraw := []byte(readlineRedraw(1, bashPrompt))
	if got := term.takePromptRedraw(redraw); string(got) != string(redraw) {
		t.Fatalf("redraw rewritten without a live prompt marker: %q", got)
	}
}

// A background job can start its output with the same carriage return and
// erase; without the repaint's prompt mark it is not readline.
func TestBashRedrawRequiresPromptMark(t *testing.T) {
	term := newPromptTerminal(40, 10)
	feedOutput(term, bashPrompt)
	term.resizeEmulator(20, 10)

	status := []byte("\r\x1b[K\x1b[Aprogress 50%")
	if got := term.takePromptRedraw(status); string(got) != string(status) {
		t.Fatalf("background output rewritten as a prompt repaint: %q", got)
	}
}
