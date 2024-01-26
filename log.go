package mirageecs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"path"
	"runtime"
	"strings"
	"sync"
)

const LogTimeFormat = "2006-01-02T15:04:05.999Z07:00"

var LogLevel = new(slog.LevelVar)

func f(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}

func SetLogLevel(l string) {
	switch strings.ToLower(l) {
	case "debug":
		LogLevel.Set(slog.LevelDebug)
	case "info":
		LogLevel.Set(slog.LevelInfo)
	case "warn":
		LogLevel.Set(slog.LevelWarn)
	case "error":
		LogLevel.Set(slog.LevelError)
	default:
		slog.Warn(f("invalid log level %s. using info", l))
		LogLevel.Set(slog.LevelInfo)
	}
}

type logHandler struct {
	opts         *slog.HandlerOptions
	preformatted []byte
	mu           *sync.Mutex
	w            io.Writer
}

func NewLogHandler(w io.Writer, opts *slog.HandlerOptions) slog.Handler {
	return &logHandler{
		opts: opts,
		mu:   new(sync.Mutex),
		w:    w,
	}
}

func (h *logHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

func (h *logHandler) Handle(ctx context.Context, record slog.Record) error {
	buf := bytes.NewBuffer(nil)
	fmt.Fprint(buf, record.Time.Format(LogTimeFormat))
	fmt.Fprintf(buf, " [%s]", strings.ToLower(record.Level.String()))
	if h.opts.AddSource {
		frame, _ := runtime.CallersFrames([]uintptr{record.PC}).Next()
		fmt.Fprintf(buf, " [%s:%d]", path.Base(frame.File), frame.Line)
	}
	if len(h.preformatted) > 0 {
		buf.Write(h.preformatted)
	}
	record.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(buf, " [%s:%v]", a.Key, a.Value)
		return true
	})
	fmt.Fprintf(buf, " %s\n", record.Message)
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(buf.Bytes())
	return err
}

func (h *logHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	preformatted := []byte{}
	for _, a := range attrs {
		preformatted = append(preformatted, fmt.Sprintf(" [%s:%v]", a.Key, a.Value)...)
	}
	return &logHandler{
		opts:         h.opts,
		preformatted: preformatted,
		mu:           h.mu,
		w:            h.w,
	}
}

func (h *logHandler) WithGroup(group string) slog.Handler {
	return h
}
