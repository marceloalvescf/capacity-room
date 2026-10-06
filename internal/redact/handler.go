package redact

import (
	"context"
	"fmt"
	"log/slog"
)

// Handler redacts the message and every attribute of a record, those bound
// by WithAttrs included, with the secrets the redactor knows when the
// record is handled, and passes the record to the inner handler.
type Handler struct {
	redactor *Redactor
	inner    slog.Handler
	// bound holds the WithAttrs and WithGroup calls in order, replayed on
	// the inner handler at each Handle.
	bound []binding
}

// binding is one WithGroup call (group) or one WithAttrs call (attrs).
type binding struct {
	group string
	attrs []slog.Attr
}

// NewHandler returns a Handler over inner.
func NewHandler(r *Redactor, inner slog.Handler) *Handler {
	return &Handler{redactor: r, inner: inner}
}

// Enabled reports whether the inner handler handles level.
func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle replays the bound groups and attributes on the inner handler, and
// passes it the record with its message and attributes redacted.
func (h *Handler) Handle(ctx context.Context, rec slog.Record) error {
	inner := h.inner
	for _, b := range h.bound {
		if b.group != "" {
			inner = inner.WithGroup(b.group)
		} else {
			inner = inner.WithAttrs(h.redactAttrs(b.attrs))
		}
	}
	out := slog.NewRecord(rec.Time, rec.Level, h.redactor.Text(rec.Message), rec.PC)
	rec.Attrs(func(a slog.Attr) bool {
		if !a.Equal(slog.Attr{}) {
			out.AddAttrs(h.redactAttr(a))
		}
		return true
	})
	return inner.Handle(ctx, out)
}

// WithAttrs records attrs, to be redacted and bound when a line is handled.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	return h.with(binding{attrs: append([]slog.Attr(nil), attrs...)})
}

// WithGroup records the group, to be opened when a line is handled.
func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return h.with(binding{group: name})
}

func (h *Handler) with(b binding) *Handler {
	bound := make([]binding, len(h.bound), len(h.bound)+1)
	copy(bound, h.bound)
	return &Handler{redactor: h.redactor, inner: h.inner, bound: append(bound, b)}
}

func (h *Handler) redactAttrs(attrs []slog.Attr) []slog.Attr {
	out := make([]slog.Attr, 0, len(attrs))
	for _, a := range attrs {
		if !a.Equal(slog.Attr{}) {
			out = append(out, h.redactAttr(a))
		}
	}
	return out
}

// redactAttr returns a with its value as redacted text: a string as it is,
// an error or a fmt.Stringer through fmt.Sprint (which prints <nil> for a
// nil receiver and recovers a panic), a byte slice as its text, and any
// other value of an unknown kind through its %+v form. A group is redacted
// member by member; every other kind carries no secret and passes.
func (h *Handler) redactAttr(a slog.Attr) slog.Attr {
	v := a.Value.Resolve()
	switch v.Kind() {
	case slog.KindString:
		return slog.String(a.Key, h.redactor.Text(v.String()))
	case slog.KindGroup:
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(h.redactAttrs(v.Group())...)}
	case slog.KindAny:
		switch x := v.Any().(type) {
		case error, fmt.Stringer:
			return slog.String(a.Key, h.redactor.Text(fmt.Sprint(x)))
		case []byte:
			return slog.String(a.Key, h.redactor.Text(string(x)))
		default:
			return slog.String(a.Key, h.redactor.Text(fmt.Sprintf("%+v", x)))
		}
	}
	return slog.Attr{Key: a.Key, Value: v}
}
