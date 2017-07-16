package core

import (
	"github.com/gazoon/bot_libs/logging"
	"github.com/pkg/errors"
)

type Intent struct {
	Handler *URL
	Words   []string
}

func NewIntent(handler *URL, words []string) *Intent {
	return &Intent{Handler: handler, Words: words}
}

func NewIntentStrHandler(handlerStr string, words []string) (*Intent, error) {
	handlerURL, err := NewURLFromStr(handlerStr)
	if err != nil {
		return nil, errors.Wrapf(err, "incorrect intent handler url %s", handlerStr)
	}
	return NewIntent(handlerURL, words)
}

func (i Intent) String() string {
	return logging.ObjToString(&i)
}
