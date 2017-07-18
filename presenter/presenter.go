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
	sessionStorage core.Storage
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
	session, err := uip.getOrCreateSession(ctx, msg)
	if err != nil {
		logger.Errorf("Cannot init chat session: %s", err)
		return
	}
	req := core.NewRequest(ctx, msg, session)
	uip.dispatchRequest(req)
	err=uip.sessionStorage.Save(ctx,session)
	if err!= nil {

	}
}

func (uip *UIPresenter) getPage(pageURL *core.URL) (pages.Page, error) {
	page, ok := uip.pageRegistry[pageURL.Page]
	if !ok {
		return nil, errors.Errorf("url %s leads to not known page %s", pageURL.Encode(), pageURL.Page)
	}
	return page, nil
}

func (uip *UIPresenter) dispatchRequest(req *core.Request) {
	logger := uip.GetLogger(req.Ctx)
	req.URL = req.URLFromMsgText()
	if req.URL != nil {
		logger.Infof("Request url %s from the message text", req.URL.Encode())
	}
	if req.Session.InputHandler != nil && req.URL == nil {
		logger.Infof("Session contains input handler %s, set it to the request url", req.Session.InputHandler.Encode())
		req.URL = req.Session.InputHandler
		req.Session.ResetInputHandler(req.Ctx)
	}

	if req.URL == nil {
		lastPageURL := req.Session.LastPage
		lastPage, err := uip.getPage(lastPageURL)
		if err != nil {
			logger.Errorf("Cannot get last page object: %s", err)
			return
		}
		logger := logger.WithField("last_page", lastPage.GetName())
		logger.Info("Handle intent on the last page")
		req.URL, err = lastPage.HandleIntent(req)
		if err != nil {
			logger.Errorf("Last page handle intent failed: %s", err)
		}
		logger.Infof("Request url %s from intent handling", req.URL.Encode())
	}
	for req.URL != nil {
		page, err := uip.getPage(req.URL)
		if err != nil {
			logger.Errorf("Cannot get page during request iteration: %s", err)
			return
		}
		logger.Info("Enter %s", req.URL.Encode())
		nextURL, err := page.Enter(req)
		if err != nil {
			logger.Errorf("Page %s failed: %s", page.GetName(), err)
			return
		}
		req.Session.SetLastPage(req.Ctx, req.URL)
		req.URL = nextURL
	}
	logger.Info("Pages iteration is successfully over")
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
	if session.LastPage == nil {
		session.LastPage = core.DefaultPageURL
	}
	return session, nil
}
