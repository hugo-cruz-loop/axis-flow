// Package logging centralizes structured application logging and output formats.
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// FormatJSON emits native slog JSON for log collectors and APMs.
	FormatJSON = "json"
	// FormatLogfmt emits human-readable local/file logs with a fixed prefix.
	FormatLogfmt = "logfmt"
)

// New creates the application logger. Business code should depend on slog's
// structured API; switching the output format is a configuration concern.
func New(w io.Writer, format string, level slog.Level) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: redactAttr,
	}

	if strings.EqualFold(format, FormatJSON) {
		return slog.New(slog.NewJSONHandler(w, opts))
	}

	return slog.New(newLogfmtHandler(w, opts))
}

func redactAttr(_ []string, a slog.Attr) slog.Attr {
	if isSensitiveKey(a.Key) {
		return slog.String(a.Key, "[REDACTED]")
	}
	return a
}

func isSensitiveKey(key string) bool {
	key = strings.ToLower(key)
	sensitiveParts := []string{
		"authorization",
		"cookie",
		"password",
		"passwd",
		"secret",
		"token",
		"refresh_token",
		"access_token",
		"jwt",
		"dsn",
		"payload",
	}
	for _, part := range sensitiveParts {
		if strings.Contains(key, part) {
			return true
		}
	}
	return false
}

type logfmtHandler struct {
	out   io.Writer
	opts  *slog.HandlerOptions
	attrs []slog.Attr
	group string
	mu    *sync.Mutex
}

func newLogfmtHandler(out io.Writer, opts *slog.HandlerOptions) *logfmtHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &logfmtHandler{out: out, opts: opts, mu: &sync.Mutex{}}
}

func (h *logfmtHandler) Enabled(_ context.Context, level slog.Level) bool {
	min := slog.LevelInfo
	if h.opts != nil && h.opts.Level != nil {
		min = h.opts.Level.Level()
	}
	return level >= min
}

func (h *logfmtHandler) Handle(_ context.Context, r slog.Record) error {
	attrs := make([]slog.Attr, 0, len(h.attrs)+r.NumAttrs())
	attrs = append(attrs, h.attrs...)
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a)
		return true
	})

	var b strings.Builder
	b.WriteString("[")
	b.WriteString(r.Time.Format(time.RFC3339Nano))
	b.WriteString("] [")
	b.WriteString(formatLevel(r.Level))
	b.WriteString("] [")
	b.WriteString(r.Message)
	b.WriteString("]")

	for _, a := range attrs {
		h.appendAttr(&b, nil, a)
	}
	b.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.out, b.String())
	return err
}

func (h *logfmtHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &clone
}

func (h *logfmtHandler) WithGroup(name string) slog.Handler {
	clone := *h
	if clone.group == "" {
		clone.group = name
	} else {
		clone.group += "." + name
	}
	return &clone
}

func (h *logfmtHandler) appendAttr(b *strings.Builder, groups []string, a slog.Attr) {
	if a.Equal(slog.Attr{}) {
		return
	}
	a.Value = a.Value.Resolve()
	key := a.Key
	if h.group != "" {
		key = h.group + "." + key
	}
	if len(groups) > 0 {
		key = strings.Join(append(groups, key), ".")
	}

	if h.opts.ReplaceAttr != nil {
		a = h.opts.ReplaceAttr(groups, a)
		a.Value = a.Value.Resolve()
	}

	if a.Value.Kind() == slog.KindGroup {
		for _, child := range a.Value.Group() {
			h.appendAttr(b, append(groups, a.Key), child)
		}
		return
	}

	b.WriteByte(' ')
	b.WriteString(key)
	b.WriteByte('=')
	b.WriteString(logfmtValue(a.Value))
}

func formatLevel(level slog.Level) string {
	switch {
	case level <= slog.LevelDebug:
		return "DEBUG"
	case level >= slog.LevelError:
		return "ERROR"
	case level >= slog.LevelWarn:
		return "WARN "
	default:
		return "INFO "
	}
}

func logfmtValue(v slog.Value) string {
	switch v.Kind() {
	case slog.KindString:
		return quoteLogfmt(v.String())
	case slog.KindBool:
		return strconv.FormatBool(v.Bool())
	case slog.KindInt64:
		return strconv.FormatInt(v.Int64(), 10)
	case slog.KindUint64:
		return strconv.FormatUint(v.Uint64(), 10)
	case slog.KindFloat64:
		return strconv.FormatFloat(v.Float64(), 'f', -1, 64)
	case slog.KindDuration:
		return quoteLogfmt(v.Duration().String())
	case slog.KindTime:
		return quoteLogfmt(v.Time().Format(time.RFC3339Nano))
	default:
		return quoteLogfmt(fmt.Sprint(v.Any()))
	}
}

func quoteLogfmt(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, " \t\n\r\"") {
		return strconv.Quote(s)
	}
	return s
}
