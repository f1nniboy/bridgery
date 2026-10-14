package media

import (
	"context"
	"testing"

	"github.com/f1nniboy/bridgery/internal/core"
)

type fakeStore struct {
	have map[string]bool
	puts int
}

func (s *fakeStore) Get(_ context.Context, key, name string) (string, bool) {
	return "stored/" + key[:8] + "/" + name, s.have[key]
}

func (s *fakeStore) Put(_ context.Context, key, name, _ string, _ []byte) (string, error) {
	s.puts++
	s.have[key] = true
	return "stored/" + key[:8] + "/" + name, nil
}

func TestResolver(t *testing.T) {
	ctx := context.Background()

	t.Run("public url passes through", func(t *testing.T) {
		r := &resolver{}
		got, err := r.URL(ctx, core.File{URL: "https://x/a.png", Public: true})
		if err != nil || got != "https://x/a.png" {
			t.Errorf("got %q, %v", got, err)
		}
	})

	t.Run("no backend resolves to nothing", func(t *testing.T) {
		r := &resolver{}
		got, err := r.URL(ctx, core.File{Data: []byte("x")})
		if err != nil || got != "" {
			t.Errorf("got %q, %v", got, err)
		}
	})

	t.Run("empty file resolves to nothing", func(t *testing.T) {
		r := &resolver{backend: &fakeStore{have: map[string]bool{}}}
		if got, _ := r.URL(ctx, core.File{}); got != "" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("bytes are stored once by content", func(t *testing.T) {
		s := &fakeStore{have: map[string]bool{}}
		r := &resolver{backend: s}
		f := core.File{Name: "a.txt", Data: []byte("hello")}
		first, err := r.URL(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		second, _ := r.URL(ctx, f)
		if first != second || s.puts != 1 {
			t.Errorf("urls %q / %q, puts %d", first, second, s.puts)
		}
	})

	t.Run("key hit skips the fetch", func(t *testing.T) {
		s := &fakeStore{have: map[string]bool{hashHex([]byte("k")): true}}
		r := &resolver{backend: s}
		got, err := r.URL(ctx, core.File{Name: "a.png", URL: "http://unreachable.invalid/a.png", Key: "k"})
		if err != nil || got == "" || s.puts != 0 {
			t.Errorf("got %q, %v, puts %d", got, err, s.puts)
		}
	})
}

func TestSafeName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"photo.png", "photo.png"},
		{"my photo (1).png", "my_photo_1_.png"},
		{"../../etc/passwd", "passwd"},
		{"a/b/c.txt", "c.txt"},
		{"..", "file"},
		{"", "file"},
		{"日本.png", "_.png"},
	}
	for _, c := range cases {
		if got := safeName(c.in); got != c.want {
			t.Errorf("safeName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
