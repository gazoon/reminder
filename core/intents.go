package core

import (
	"github.com/gazoon/bot_libs/logging"
)

type Intent struct {
	Handler *URL
	Words   []string
}

func NewIntent(handler *URL, words []string) *Intent {
	return &Intent{Handler: handler, Words: words}
}

func (i Intent) String() string {
	return logging.ObjToString(&i)
}
