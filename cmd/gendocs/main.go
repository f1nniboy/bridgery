package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/f1nniboy/bridgery/internal/core"
	"github.com/f1nniboy/bridgery/internal/platforms"
)

const dir = "docs/platforms"

const marker = "<!-- This file was generated. Do not edit. -->\n\n"

func main() {
	names := make([]string, 0, len(platforms.Registry))
	for name := range platforms.Registry {
		names = append(names, name)
	}
	slices.Sort(names)

	for _, name := range names {
		d := filepath.Join(dir, name)
		if err := os.MkdirAll(d, 0o755); err != nil {
			fail(err)
		}
		if err := os.WriteFile(filepath.Join(d, "README.md"), []byte(page(name)), 0o644); err != nil {
			fail(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(index(names)), 0o644); err != nil {
		fail(err)
	}
}

func page(name string) string {
	ex := platforms.Example(name)
	var b strings.Builder
	b.WriteString(marker)
	fmt.Fprintf(&b, "# %s\n\n", ex.Name())
	if guides := guides(filepath.Join(dir, name)); len(guides) > 0 {
		for _, g := range guides {
			fmt.Fprintf(&b, "- [%s](%s)\n", title(strings.ToLower(strings.TrimSuffix(g, ".md"))), g)
		}
		b.WriteString("\n")
	}
	b.WriteString("## Features\n\n")
	b.WriteString(features(ex.Caps()))
	b.WriteString("\n## Configuration\n\n```toml\n")
	b.WriteString(configBlock(name, ex))
	b.WriteString("\n```\n")
	return b.String()
}

func features(c core.CapSet) string {
	var b strings.Builder
	b.WriteString("| Feature | Status |\n|---|:-:|\n")
	for _, cp := range core.AllCaps {
		fmt.Fprintf(&b, "| %s | %s |\n", title(cp), mark(c.Has(cp)))
	}
	return b.String()
}

func configBlock(name string, bridge core.Bridge) string {
	b, err := toml.Marshal(bridge)
	if err != nil {
		fail(err)
	}
	return fmt.Sprintf("[accounts.%s.my%s]\n%s", name, name, strings.TrimSpace(string(b)))
}

func index(names []string) string {
	var b strings.Builder
	b.WriteString(marker)
	b.WriteString("# Platforms\n\n")
	b.WriteString("**bridgery** bridges the platforms below. Open one for its capabilities and how to set it setup.\n\n")
	for _, name := range names {
		fmt.Fprintf(&b, "- **[%s](%s)**\n", platforms.Example(name).Name(), name)
		for _, g := range guides(filepath.Join(dir, name)) {
			fmt.Fprintf(&b, "  - [%s](%s/%s)\n", title(strings.ToLower(strings.TrimSuffix(g, ".md"))), name, g)
		}
	}
	return b.String()
}

func guides(d string) []string {
	entries, err := os.ReadDir(d)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if n := e.Name(); !e.IsDir() && n != "README.md" && strings.HasSuffix(n, ".md") {
			out = append(out, n)
		}
	}
	slices.Sort(out)
	return out
}

func title(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func mark(ok bool) string {
	if ok {
		return "✅"
	}
	return "❌"
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
