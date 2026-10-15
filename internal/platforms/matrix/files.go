package matrix

import (
	"cmp"
	"context"
	"net/http"
	"strings"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/f1nniboy/bridgery/internal/core"
)

func (b *Bridge) fetch(uri id.ContentURIString) func(context.Context) ([]byte, error) {
	cli := b.client
	return func(ctx context.Context) ([]byte, error) {
		mxc, err := uri.Parse()
		if err != nil {
			return nil, err
		}
		return cli.DownloadBytes(ctx, mxc)
	}
}

func upload(ctx context.Context, cli *mautrix.Client, f core.File) (*event.MessageEventContent, error) {
	data, err := f.Bytes(ctx)
	if err != nil {
		return nil, err
	}
	mime := f.MIME
	if mime == "" {
		mime = http.DetectContentType(data)
	}
	name := cmp.Or(f.Name, "file")
	resp, err := cli.UploadBytesWithName(ctx, data, mime, name)
	if err != nil {
		return nil, err
	}
	return &event.MessageEventContent{
		MsgType:  msgType(mime),
		Body:     name,
		FileName: name,
		URL:      resp.ContentURI.CUString(),
		Info:     &event.FileInfo{MimeType: mime, Size: len(data)},
		Mentions: &event.Mentions{},
	}, nil
}

func msgType(mime string) event.MessageType {
	switch kind, _, _ := strings.Cut(mime, "/"); kind {
	case "image":
		return event.MsgImage
	case "video":
		return event.MsgVideo
	case "audio":
		return event.MsgAudio
	}
	return event.MsgFile
}
