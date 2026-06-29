package cli

import (
	"bytes"
	"io"
	"os"
	"testing"
)

// captureOutput captures stdout output of a function
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestDisplayProgressBarOutput(t *testing.T) {
	// 50% progress
	// barLength is 40. 50% is 20 chars.
	expected := "  [████████████████████░░░░░░░░░░░░░░░░░░░░] 50%\n"
	output := captureOutput(func() {
		displayProgressBar(50)
	})

	if output != expected {
		t.Errorf("Expected:\n%q\nGot:\n%q", expected, output)
	}

	// > 100% progress (should clamp to 100%)
	expected = "  [████████████████████████████████████████] 100%\n"
	output = captureOutput(func() {
		displayProgressBar(150)
	})
	if output != expected {
		t.Errorf("Expected:\n%q\nGot:\n%q", expected, output)
	}

	// < 0% progress (should clamp to 0%)
	expected = "  [░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░] 0%\n"
	output = captureOutput(func() {
		displayProgressBar(-10)
	})
	if output != expected {
		t.Errorf("Expected:\n%q\nGot:\n%q", expected, output)
	}

	// 0% progress
	expected = "  [░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░] 0%\n"
	output = captureOutput(func() {
		displayProgressBar(0)
	})
	if output != expected {
		t.Errorf("Expected:\n%q\nGot:\n%q", expected, output)
	}

	// 100% progress
	expected = "  [████████████████████████████████████████] 100%\n"
	output = captureOutput(func() {
		displayProgressBar(100)
	})
	if output != expected {
		t.Errorf("Expected:\n%q\nGot:\n%q", expected, output)
	}
}
