package core

import (
	"context"
	"github.com/gazoon/bot_libs/logging"
)

var (
	gLogger = logging.WithPackage("core")
)

type Session struct {
	CurrentPageName string
	LocalIntents    []*Intent
	InputHandler    string
	ChatID          int
}

func (s *Session) AddIntent(intent *Intent) {
	s.LocalIntents = append(s.LocalIntents, intent)
}

func (s *Session) SetInputHandler(ctx context.Context, handler string) {
	logger := logging.FromContextAndBase(ctx, gLogger)
	logger.Info("Change input handler new: %s, old: %s", handler, s.InputHandler)
	s.InputHandler = handler
}
