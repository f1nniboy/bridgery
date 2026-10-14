package span

import (
	"fmt"
	"strings"
)

type Kind string

const (
	KindText    Kind = "text"
	KindMention Kind = "mention"
)

type Span struct {
	Kind Kind   `json:"kind" luar:"kind"`
	Text string `json:"text,omitzero" luar:"text"`
}

func (sp Span) Display() string {
	switch sp.Kind {
	case KindMention:
		return "@" + sp.Text
	default:
		return sp.Text
	}
}

type Spans []Span

func Plain(s string) Spans {
	if s == "" {
		return nil
	}
	return Spans{{Kind: KindText, Text: s}}
}

func Mention(display string) Span { return Span{Kind: KindMention, Text: display} }

func Of(parts ...any) Spans {
	var out Spans
	for _, p := range parts {
		switch v := p.(type) {
		case string:
			out = append(out, Plain(v)...)
		case Spans:
			out = append(out, v...)
		case Span:
			out = append(out, v)
		default:
			panic(fmt.Sprintf("unsupported part %T, want a string or spans", p))
		}
	}
	return out
}

func (s Spans) Text() string {
	if len(s) == 1 {
		return s[0].Display()
	}
	var b strings.Builder
	for _, sp := range s {
		b.WriteString(sp.Display())
	}
	return b.String()
}
