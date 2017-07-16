package presenter

import (
	"context"
	"github.com/gazoon/bot_libs/logging"
	"github.com/gazoon/bot_libs/messenger"
	"github.com/gazoon/bot_libs/queue/messages"
	"github.com/pkg/errors"
	"reminder/core"
	"reminder/core/pages"
)

var (
	gLogger = logging.WithPackage("ui_presenter")
)

type PresenterSettings struct {
	SupportGroups       bool
	OnlyAppealsInGroups bool
}

type UIPresenter struct {
	*logging.ObjectLogger
	messenger      messenger.Messenger
	sessionStorage core.SessionStorage
	pageRegistry   map[string]pages.Page
	//globalIntents []*core.Intent
	settings *PresenterSettings
}

func (uip *UIPresenter) needSkip(ctx context.Context, msg *msgsqueue.Message) bool {
	if msg.Chat.IsPrivate {
		return false
	}
	logger := uip.GetLogger(ctx).WithField("chat_id", msg.Chat.ID)
	if !uip.settings.SupportGroups {
		logger.Info("chat is group, skip")
		return true
	}
	if uip.settings.OnlyAppealsInGroups && !msg.IsAppeal {
		logger.WithField("msg_text", msg.Text).Info("message text doesn't contain an appeal to the bot, skip")
		return true
	}
	return false
}

func (uip *UIPresenter) OnMessage(ctx context.Context, msg *msgsqueue.Message) {
	logger := uip.GetLogger(ctx)
	if uip.needSkip(ctx, msg) {
		return
	}
	session,err:=uip.getOrCreateSession(ctx,msg)
	if err!= nil {
		logger.Errorf("Cannot init chat session: %s",err)
		return
	}
	req:=core.NewRequest(ctx,msg,session)

}

func (uip *UIPresenter) getOrCreateSession(ctx context.Context, msg *msgsqueue.Message) (*core.Session, error) {
	session, err := uip.sessionStorage.Get(ctx, msg.Chat.ID)
	if err != nil {
		return nil, errors.Wrap(err, "storage get")
	}
	logger := uip.GetLogger(ctx)
	if session == nil {
		logger.WithField("chat_id", msg.Chat.ID).Info("session doesn't exist for chat, init a new one")
		session = core.NewSession(msg.Chat.ID)
	} else {
		logger.WithField("session", session).Info("chat session from storage")
	}
	return session, nil
}
