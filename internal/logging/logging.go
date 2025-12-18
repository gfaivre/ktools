package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
)

var logger *slog.Logger

func init() {
	// Default: only errors (no output for info/debug)
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
	slog.SetDefault(logger)
}

// compactHandler is a minimal handler for CLI output
type compactHandler struct {
	w     io.Writer
	level slog.Level
}

func (h *compactHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *compactHandler) Handle(_ context.Context, r slog.Record) error {
	// Format: [HH:MM:SS] msg key=value ...
	ts := r.Time.Format("15:04:05")
	fmt.Fprintf(h.w, "[%s] %s", ts, r.Message)

	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(h.w, " %s=%v", a.Key, a.Value)
		return true
	})
	fmt.Fprintln(h.w)
	return nil
}

func (h *compactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *compactHandler) WithGroup(name string) slog.Handler {
	return h
}

// SetVerbose enables debug-level logging with compact output
func SetVerbose(verbose bool) {
	if verbose {
		logger = slog.New(&compactHandler{w: os.Stderr, level: slog.LevelDebug})
	} else {
		logger = slog.New(&compactHandler{w: os.Stderr, level: slog.LevelError})
	}
	slog.SetDefault(logger)
}

// Debug logs at debug level
func Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
}

// Info logs at info level
func Info(msg string, args ...any) {
	slog.Info(msg, args...)
}

// Warn logs at warn level
func Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
}

// Error logs at error level
func Error(msg string, args ...any) {
	slog.Error(msg, args...)
}
