package xmpp

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"net/url"
	"path"

	"mellium.im/xmpp/jid"
	"mellium.im/xmpp/stanza"

	"github.com/f1nniboy/bridgery/internal/core"
)

const (
	nsReply = "urn:xmpp:reply:0" // https://xmpp.org/extensions/xep-0461.html
	nsVCard = "vcard-temp"       // https://xmpp.org/extensions/xep-0054.html
)

// https://xmpp.org/extensions/xep-0424.html
type retract struct {
	XMLName xml.Name `xml:"urn:xmpp:message-retract:1 retract"`
	ID      string   `xml:"id,attr"`
}

// https://xmpp.org/extensions/xep-0334.html
type storeHint struct {
	XMLName xml.Name `xml:"urn:xmpp:hints store"`
}

// https://xmpp.org/extensions/xep-0461.html
type reply struct {
	XMLName xml.Name `xml:"urn:xmpp:reply:0 reply"`
	To      string   `xml:"to,attr,omitempty"`
	ID      string   `xml:"id,attr"`
}

// https://xmpp.org/extensions/xep-0428.html
type fallback struct {
	XMLName xml.Name       `xml:"urn:xmpp:fallback:0 fallback"`
	For     string         `xml:"for,attr"`
	Body    []fallbackBody `xml:"body"`
}

type fallbackBody struct {
	Start *int `xml:"start,attr"`
	End   *int `xml:"end,attr"`
}

// https://xmpp.org/extensions/xep-0359.html
type originID struct {
	XMLName xml.Name `xml:"urn:xmpp:sid:0 origin-id"`
	ID      string   `xml:"id,attr"`
}

// https://xmpp.org/extensions/xep-0359.html
type stanzaID struct {
	XMLName xml.Name `xml:"urn:xmpp:sid:0 stanza-id"`
	ID      string   `xml:"id,attr"`
	By      string   `xml:"by,attr"`
}

// https://xmpp.org/extensions/xep-0066.html
type oob struct {
	URL string `xml:"url"`
}

// https://xmpp.org/extensions/xep-0380.html
type encryption struct {
	XMLName xml.Name `xml:"urn:xmpp:eme:0 encryption"`
}

// https://xmpp.org/extensions/xep-0085.html
type chatState struct{}

type outMessage struct {
	stanza.Message
	Body      string     `xml:"body,omitempty"`
	OriginID  *originID  `xml:",omitempty"`
	Reply     *reply     `xml:",omitempty"`
	Retract   *retract   `xml:",omitempty"`
	Store     *storeHint `xml:",omitempty"`
	Composing *chatState `xml:"http://jabber.org/protocol/chatstates composing,omitempty"`
	Active    *chatState `xml:"http://jabber.org/protocol/chatstates active,omitempty"`
}

type inMessage struct {
	stanza.Message
	Body       string      `xml:"body"`
	OriginID   originID    `xml:"urn:xmpp:sid:0 origin-id"`
	StanzaIDs  []stanzaID  `xml:"urn:xmpp:sid:0 stanza-id"`
	Reply      reply       `xml:"urn:xmpp:reply:0 reply"`
	Retract    retract     `xml:"urn:xmpp:message-retract:1 retract"`
	Fallback   []fallback  `xml:"urn:xmpp:fallback:0 fallback"`
	OOB        []oob       `xml:"jabber:x:oob x"`
	Encryption *encryption `xml:"urn:xmpp:eme:0 encryption"`
	Composing  *chatState  `xml:"http://jabber.org/protocol/chatstates composing"`
}

func (m inMessage) stanzaID(room jid.JID) string {
	for _, s := range m.StanzaIDs {
		if by, err := jid.Parse(s.By); err == nil && by.Equal(room) {
			return s.ID
		}
	}
	return ""
}

func (m inMessage) files() []core.File {
	var out []core.File
	for _, o := range m.OOB {
		u, err := url.Parse(o.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			continue
		}
		name := path.Base(u.Path)
		if name == "." || name == "/" {
			name = "file"
		}
		out = append(out, core.File{Name: name, URL: o.URL, Public: true})
	}
	return out
}

type inPresence struct {
	stanza.Presence
	Error *presenceError `xml:"error"`
	VCard *vcardX        `xml:"vcard-temp:x:update x"`
	MUC   *mucUser       `xml:"http://jabber.org/protocol/muc#user x"`
}

type mucUser struct {
	Status []mucStatus `xml:"status"`
}

// https://xmpp.org/extensions/xep-0045.html#statuscodes
type mucStatus struct {
	Code int `xml:"code,attr"`
}

func (m *mucUser) removal() (code int, permanent bool) {
	if m == nil {
		return 0, false
	}
	for _, s := range m.Status {
		switch s.Code {
		case 301, 321, 322: // ban, affiliation change, members-only
			return s.Code, true
		case 307, 332, 333: // kick, shutdown, error
			return s.Code, false
		}
	}
	return 0, false
}

func (m *mucUser) selfJoined() bool {
	if m == nil {
		return false
	}
	for _, s := range m.Status {
		if s.Code == 110 {
			return true
		}
	}
	return false
}

// https://xmpp.org/extensions/xep-0153.html
type vcardX struct {
	XMLName xml.Name `xml:"vcard-temp:x:update x"`
	Photo   string   `xml:"photo"`
}

type inIQ struct {
	stanza.IQ
	VCard *vcardQuery `xml:"vcard-temp vCard"`
}

type vcardQuery struct{}

// https://xmpp.org/extensions/xep-0054.html
type vCard struct {
	XMLName xml.Name    `xml:"vcard-temp vCard"`
	Photo   *vCardPhoto `xml:"PHOTO"`
}

type vCardPhoto struct {
	Type   string `xml:"TYPE"`
	BinVal string `xml:"BINVAL"`
}

func vcardPayload(mime string, data []byte) xml.TokenReader {
	v := vCard{Photo: &vCardPhoto{Type: mime, BinVal: base64.StdEncoding.EncodeToString(data)}}
	var buf bytes.Buffer
	_ = xml.NewEncoder(&buf).Encode(&v)
	return xml.NewDecoder(&buf)
}

// https://datatracker.ietf.org/doc/html/rfc6120#section-8.3.3
type presenceError struct {
	Type      string      `xml:"type,attr"`
	Condition conditionEl `xml:",any"`
	Text      string      `xml:"urn:ietf:params:xml:ns:xmpp-stanzas text"`
}

type conditionEl struct {
	XMLName xml.Name
}

func (e *presenceError) reason() string {
	if e == nil {
		return "rejected"
	}
	if c := e.Condition.XMLName.Local; c != "" {
		return c
	}
	if e.Text != "" {
		return e.Text
	}
	return e.Type
}

// https://xmpp.org/extensions/xep-0045.html#enter-errorcodes
func (e *presenceError) permanent() bool {
	if e == nil {
		return false
	}
	switch e.Condition.XMLName.Local {
	case "forbidden", "not-authorized", "registration-required", "not-allowed", "not-acceptable", "item-not-found":
		return true
	}
	return false
}

// https://xmpp.org/extensions/xep-0045.html#enter
type mucX struct {
	XMLName  xml.Name    `xml:"http://jabber.org/protocol/muc x"`
	History  *mucHistory `xml:"history,omitempty"`
	Password string      `xml:"password,omitempty"`
}

type mucHistory struct {
	MaxStanzas int `xml:"maxstanzas,attr"`
}

type outPresence struct {
	stanza.Presence
	X     *mucX   `xml:",omitempty"`
	VCard *vcardX `xml:",omitempty"`
}

func stripFallback(text string, fbs []fallback) string {
	runes := []rune(text)
	var drop []bool
	for _, fb := range fbs {
		if fb.For != nsReply {
			continue
		}
		for _, b := range fb.Body {
			if b.Start == nil || b.End == nil {
				return "" // whole body is reply fallback
			}
			s, e := *b.Start, *b.End
			if s < 0 || e <= s || e > len(runes) {
				continue
			}
			if drop == nil {
				drop = make([]bool, len(runes))
			}
			for i := s; i < e; i++ {
				drop[i] = true
			}
		}
	}
	if drop == nil {
		return text
	}
	out := make([]rune, 0, len(runes))
	for i, r := range runes {
		if !drop[i] {
			out = append(out, r)
		}
	}
	return string(out)
}
