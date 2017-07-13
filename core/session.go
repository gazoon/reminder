package core

import (
	"context"
	"encoding/json"
	"github.com/gazoon/bot_libs/logging"
	"github.com/gazoon/bot_libs/queue/messages"
	"github.com/pkg/errors"
	"reminder/models"
	"sync"
)

var (
	gLogger = logging.WithPackage("core")
)

type Request struct {
	Session *Session
	Ctx     context.Context
	Msg     *msgsqueue.Message
	Chat    *models.Chat
	User    *models.User
}

type Session struct {
	ID              string
	CurrentPageName string
	LocalIntents    []*Intent
	InputHandler    string
	ChatID          int
	PagesStates     map[string]map[string]interface{}
	GlobalState     map[string]interface{}
}

func (s *Session) AddIntent(intent *Intent) {
	s.LocalIntents = append(s.LocalIntents, intent)
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
