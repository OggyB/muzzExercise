package logger

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/oggyb/muzz-exercise/internal/config"
)

// captureOutput redirects stdout to a buffer during f()
func captureOutput(t *testing.T, f func()) string {
	t.Helper()

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	_ = r.Close()

	return buf.String()
}

func TestLogger_TextFormat(t *testing.T) {
	out := captureOutput(t, func() {
		InitFromConfig(config.Config{
			Log: struct {
				Level     string
				Format    string
				Component string
				Source    bool
			}{
				Level:     "debug",
				Format:    "text",
				Component: "test",
				Source:    false,
			},
		})
		Info("hello muzz", "key", "value")
	})

	if !strings.Contains(out, "hello muzz") {
		t.Errorf("expected message, got: %s", out)
	}
	if !strings.Contains(out, "component=test") {
		t.Errorf("expected component field, got: %s", out)
	}
	if !strings.Contains(out, "key=value") {
		t.Errorf("expected structured field, got: %s", out)
	}
}

func TestLogger_JSONFormat(t *testing.T) {
	out := captureOutput(t, func() {
		InitFromConfig(config.Config{
			Log: struct {
				Level     string
				Format    string
				Component string
				Source    bool
			}{
				Level:     "info",
				Format:    "json",
				Component: "json_test",
				Source:    false,
			},
		})
		Info("json log", "foo", "bar")
	})

	if !strings.Contains(out, `"msg":"json log"`) {
		t.Errorf("expected JSON message, got: %s", out)
	}
	if !strings.Contains(out, `"component":"json_test"`) {
		t.Errorf("expected component in JSON, got: %s", out)
	}
	if !strings.Contains(out, `"foo":"bar"`) {
		t.Errorf("expected structured field in JSON, got: %s", out)
	}
}

func TestLogger_LevelFilter(t *testing.T) {
	out := captureOutput(t, func() {
		InitFromConfig(config.Config{
			Log: struct {
				Level     string
				Format    string
				Component string
				Source    bool
			}{
				Level:  "error",
				Format: "text",
			},
		})
		Info("should not appear")
		Error("should appear")
	})

	if strings.Contains(out, "should not appear") {
		t.Errorf("info log should not appear, got: %s", out)
	}
	if !strings.Contains(out, "should appear") {
		t.Errorf("error log should appear, got: %s", out)
	}
}

func TestLogger_WithAddsFields(t *testing.T) {
	out := captureOutput(t, func() {
		InitFromConfig(config.Config{
			Log: struct {
				Level     string
				Format    string
				Component string
				Source    bool
			}{
				Level:  "debug",
				Format: "text",
			},
		})
		log := With("req_id", "123")
		log.Info("processing request")
	})

	if !strings.Contains(out, "req_id=123") {
		t.Errorf("expected req_id field, got: %s", out)
	}
}

func TestLogger_InitFromConfig(t *testing.T) {
	out := captureOutput(t, func() {
		InitFromConfig(config.Config{
			Log: struct {
				Level     string
				Format    string
				Component string
				Source    bool
			}{
				Level:     "debug",
				Format:    "json",
				Component: "cfg_test",
				Source:    true,
			},
		})
		Debug("cfg-based log")
	})

	if !strings.Contains(out, `"msg":"cfg-based log"`) {
		t.Errorf("expected config-based JSON log, got: %s", out)
	}
	if !strings.Contains(out, `"component":"cfg_test"`) {
		t.Errorf("expected component from config, got: %s", out)
	}
}
