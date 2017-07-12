package core

import (
	"context"
	"github.com/gazoon/bot_libs/logging"
	"github.com/gazoon/bot_libs/queue/messages"
	"reminder/models"
)

var (
	gLogger = logging.WithPackage("core")
)

type Request struct {
	Session *Session
	Ctx     context.Context
	Msg     *msgsqueue.Message
	Chat    *models.Chat
	User *models.User
}

type Session struct {
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
