package irc

import "strings"

func (b *Bridge) remember(name string) {
	b.chanMu.Lock()
	defer b.chanMu.Unlock()
	b.channels[b.fold(name)] = name
}

func (b *Bridge) channel(name string) string {
	b.chanMu.Lock()
	defer b.chanMu.Unlock()
	if configured, ok := b.channels[b.fold(name)]; ok {
		return configured
	}
	return name
}

func (b *Bridge) fold(name string) string {
	return casefold(name, b.conn.ISupport()["CASEMAPPING"])
}

// https://modern.ircdocs.horse/#casemapping-parameter
func casefold(s, mapping string) string {
	if mapping == "rfc7613" || mapping == "rfc8265" {
		return strings.ToLower(s)
	}
	return strings.Map(func(r rune) rune {
		switch {
		case 'A' <= r && r <= 'Z':
			return r + ('a' - 'A')
		case mapping == "ascii":
			return r
		case r == '[':
			return '{'
		case r == ']':
			return '}'
		case r == '\\':
			return '|'
		case r == '~' && mapping != "rfc1459-strict":
			return '^'
		}
		return r
	}, s)
}
