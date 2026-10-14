package platforms

import (
	"github.com/f1nniboy/bridgery/internal/core"
	"github.com/f1nniboy/bridgery/internal/platforms/console"
	"github.com/f1nniboy/bridgery/internal/platforms/discord"
	"github.com/f1nniboy/bridgery/internal/platforms/irc"
	"github.com/f1nniboy/bridgery/internal/platforms/xmpp"
)

var Registry = map[string]func(*core.Common) core.Bridge{
	"discord": func(c *core.Common) core.Bridge { return &discord.Bridge{Common: c} },
	"console": func(c *core.Common) core.Bridge { return &console.Bridge{Common: c} },
	"irc":     func(c *core.Common) core.Bridge { return &irc.Bridge{Common: c} },
	"xmpp":    func(c *core.Common) core.Bridge { return &xmpp.Bridge{Common: c} },
}

func Example(name string) core.Bridge {
	if newBridge, ok := Registry[name]; ok {
		return newBridge(nil).Example()
	}
	return nil
}
