package core

import (
	"strings"

	"github.com/f1nniboy/bridgery/internal/span"
)

type Author struct {
	ID     string `luar:"id"`
	Name   string `luar:"name"`
	Avatar File   `luar:"avatar"`
}

func (a Author) Display() string {
	if a.Name != "" {
		return a.Name
	}
	return a.ID // TODO: kind of bad?
}

type Message struct {
	Author  Author     `luar:"author"`
	Content span.Spans `luar:"content"`
	Files   []File     `luar:"files"`
}

func (m *Message) Prepend(parts ...any) { m.Content = append(span.Of(parts...), m.Content...) }
func (m *Message) Append(parts ...any)  { m.Content = append(m.Content, span.Of(parts...)...) }
func (m *Message) Set(parts ...any)     { m.Content = span.Of(parts...) }

func (m Message) Excerpt(n int) span.Spans {
	var out span.Spans
	budget := n
	for _, sp := range m.Content {
		if sp.Kind != span.KindText {
			out = append(out, sp)
			continue
		}
		r := []rune(strings.ReplaceAll(sp.Text, "\n", " "))
		if len(r) > budget {
			out = append(out, span.Span{Kind: span.KindText, Text: strings.TrimRight(string(r[:budget]), " ") + "…"})
			break
		}
		budget -= len(r)
		if len(r) > 0 {
			out = append(out, span.Span{Kind: span.KindText, Text: string(r)})
		}
	}
	return out
}
