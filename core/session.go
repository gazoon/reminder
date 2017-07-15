package core

import (
	"context"
	"encoding/json"
	"net/url"
	"reminder/models"
	"strings"
	"sync"

	"github.com/gazoon/bot_libs/logging"
	"github.com/gazoon/bot_libs/queue/messages"
	"github.com/pkg/errors"
)

const (
	DefaultAction = "main"
	urlScheme     = "page"
)

var (
	gLogger = logging.WithPackage("core")
)

type URL struct {
	Page   string
	Action string
	Params map[string]string
}

func NewURL(page, action string, params map[string]string) *URL {
	if action == "" {
		action = DefaultAction
	}
	return &URL{Page: page, Action: action, Params: params}
}

func NewURLFromStr(rawurl string) (*URL, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, errors.Wrap(err, "url parsing")
	}
	if u.Scheme != urlScheme && u.Scheme != "" {
		return nil, errors.Errorf("supported scheme %s, found %s", urlScheme, u.Scheme)
	}
	queryValues := u.Query()
	params := make(map[string]string, len(queryValues))
	for k := range queryValues {
		params[k] = queryValues.Get(k)
	}
	action := strings.Trim(u.Path, "/")
	return NewURL(u.Host, action, params), nil
}

func (u *URL) Encode() string {
	queryValues := make(url.Values, len(u.Params))
	for k, v := range u.Params {
		queryValues.Set(k, v)
	}
	underlingURL := url.URL{Host: u.Page, Path: u.Action, Scheme: urlScheme, RawQuery: queryValues.Encode()}
	return underlingURL.String()
}

func (u URL) String() string {
	return logging.ObjToString(&u)
}

func (u *URL) IsRelative() bool {
	return u.Page == ""
}

type Request struct {
	Session *Session
	Ctx     context.Context
	Msg     *msgsqueue.Message
	Chat    *models.Chat
	User    *models.User
	URL     *URL
}

type Session struct {
	ID           string
	CurrentURL   *URL
	LocalIntents []*Intent
	InputHandler string
	ChatID       int
	PagesStates  map[string]map[string]interface{}
	GlobalState  map[string]interface{}
}

func (s *Session) AddIntent(words []string, handler *URL) {
	s.LocalIntents = append(s.LocalIntents, NewIntent(handler, words))
}

func (s *Session) SetInputHandler(ctx context.Context, handler string) {
	logger := logging.FromContextAndBase(ctx, gLogger)
	logger.Infof("Change input handler new: %s, old: %s", handler, s.InputHandler)
	s.InputHandler = handler
}

type SessionStorage interface {
	Get(ctx context.Context, chatID int) (*Session, error)
	Save(ctx context.Context, session *Session) error
	Delete(ctx context.Context, session *Session) error
}

type InMemorySessionStorage struct {
	mx      sync.RWMutex
	storage map[int][]byte
}

func (ms *InMemorySessionStorage) Get(ctx context.Context, chatID int) (*Session, error) {
	ms.mx.RLock()
	sessionData, ok := ms.storage[chatID]
	ms.mx.RUnlock()
	if !ok {
		return nil, nil
	}
	session := &Session{}
	err := json.Unmarshal(sessionData, session)
	if err != nil {
		return nil, errors.Wrap(err, "session data unmarshal")
	}
	return session, nil
}

func (ms *InMemorySessionStorage) Save(ctx context.Context, session *Session) error {
	sessionData, err := json.Marshal(session)
	if err != nil {
		return errors.Wrap(err, "session marshal")
	}
	ms.mx.Lock()
	ms.storage[session.ChatID] = sessionData
	ms.mx.Unlock()
	return nil
}

func (ms *InMemorySessionStorage) Delete(ctx context.Context, session *Session) error {
	ms.mx.Lock()
	defer ms.mx.Unlock()
	delete(ms.storage, session.ChatID)
	return nil
}
