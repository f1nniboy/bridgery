package core

import (
	"slices"
	"strings"

	"github.com/f1nniboy/bridgery/internal/span"
)

type Author struct {
	ID     string `json:"id,omitzero" luar:"id"`
	Name   string `json:"name,omitzero" luar:"name"`
	Avatar File   `json:"avatar,omitzero" luar:"avatar"`
}

func (a Author) Display() string {
	if a.Name != "" {
		return a.Name
	}
	return a.ID // TODO: kind of a bad fallback?
}

type Message struct {
	Author  Author     `json:"author,omitzero" luar:"author"`
	Content span.Spans `json:"content,omitzero" luar:"content"`
	Files   []File     `json:"files,omitzero" luar:"files"`
}

func (m Message) WithoutData() Message {
	m.Author.Avatar.Data, m.Author.Avatar.Fetch = nil, nil
	if len(m.Files) > 0 {
		files := slices.Clone(m.Files)
		for i := range files {
			files[i].Data, files[i].Fetch = nil, nil
		}
		m.Files = files
	}
	return m
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
