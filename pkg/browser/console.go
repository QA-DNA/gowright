package browser

import "encoding/json"

type ConsoleMessage struct {
	typ      string
	text     string
	args     []json.RawMessage
	location ConsoleMessageLocation
	page     *Page
}

type ConsoleMessageLocation struct {
	URL    string `json:"url"`
	Line   int    `json:"lineNumber"`
	Column int    `json:"columnNumber"`
}

func (cm *ConsoleMessage) Type() string                     { return cm.typ }
func (cm *ConsoleMessage) Text() string                     { return cm.text }
func (cm *ConsoleMessage) Args() []json.RawMessage          { return cm.args }
func (cm *ConsoleMessage) Location() ConsoleMessageLocation { return cm.location }
func (cm *ConsoleMessage) Page() *Page                      { return cm.page }
