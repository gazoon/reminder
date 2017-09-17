package presenter

import (
	"context"
	"reminder/core"
	"reminder/core/page"

	"github.com/gazoon/bot_libs/logging"
	"github.com/gazoon/bot_libs/messenger"
	"github.com/gazoon/bot_libs/queue/messages"
	"github.com/pkg/errors"
)

const (
	errorMessageText = "An internal bot error occurred."
)

var (
	DefaultSettings = Settings{SupportGroups: false, OnlyAppealsInGroups: true}
)

type Settings struct {
	SupportGroups       bool
	OnlyAppealsInGroups bool
}

type UIPresenter struct {
	*logging.ObjectLogger
	messenger      messenger.Messenger
	sessionStorage core.Storage
	pageRegistry   map[string]page.Page
	//globalIntents []*core.Intent
	settings *Settings
}

func New(messenger messenger.Messenger, storage core.Storage, pageRegistry map[string]page.Page,
	settings *Settings) *UIPresenter {

	logger := logging.NewObjectLogger("ui_presenter", nil)
	if settings == nil {
		settings = &DefaultSettings
	}
	return &UIPresenter{ObjectLogger: logger, messenger: messenger, sessionStorage: storage,
		pageRegistry: pageRegistry, settings: settings}
}

func (uip *UIPresenter) OnQueueMessage(ctx context.Context, msg *msgsqueue.Message) {
	if uip.needSkip(ctx, msg) {
		return
	}
	req := core.NewRequestFromQueueMsg(ctx, msg)
	ok := uip.HandleRequest(ctx, req)
	if !ok {
		uip.sendError(ctx, msg)
	}
}

func (uip *UIPresenter) HandleRequest(ctx context.Context, req *core.Request) bool {
	logger := uip.GetLogger(ctx)
	session, err := uip.getOrCreateSession(ctx, req.ChatID)
	if err != nil {
		logger.Errorf("Cannot init chat session: %s", err)
		return false
	}
	req.SetSession(session)
	ok := uip.dispatchRequest(req)
	if !ok {
		return false
	}
	return uip.saveSession(req)
}

func (uip *UIPresenter) dispatchRequest(req *core.Request) bool {
	logger := uip.GetLogger(req.Ctx)
	req.Session.ResetIntents(req.Ctx)
	if req.URL != nil {
		logger.Infof("Request url %s", req.URL.Encode())
		req.Session.ResetInputHandler(req.Ctx)
	} else if req.Session.InputHandler != nil {
		logger.Infof("Session contains input handler %s, set it to the request url", req.Session.InputHandler.Encode())
		req.URL = req.Session.InputHandler
		req.Session.ResetInputHandler(req.Ctx)
	} else {
		lastPageURL := req.Session.LastPage
		lastPage, err := uip.getPage(lastPageURL)
		if err != nil {
			logger.Errorf("Cannot get last page object: %s", err)
			return false
		}
		logger := logger.WithField("last_page", lastPage.GetName())
		logger.Info("Handle intent on the last page")
		req.URL, err = lastPage.HandleIntent(req)
		if err != nil {
			logger.Errorf("Last page handle intent failed: %s", err)
			return false
		}
		logger.Infof("Request url %s from intent handling", req.URL.Encode())
	}
	for req.URL != nil {
		pg, err := uip.getPage(req.URL)
		if err != nil {
			logger.Errorf("Cannot get page during request iteration: %s", err)
			return false
		}
		logger.Infof("Enter %s", req.URL.Encode())
		nextURL, err := pg.Enter(req)
		if err != nil {
			logger.Errorf("Page %s failed: %+v", pg.GetName(), err)
			return false
		}
		req.Session.SetLastPage(req.Ctx, req.URL)
		req.URL = nextURL
	}
	logger.Info("Pages iteration is successfully over")
	return true
}

func (uip *UIPresenter) saveSession(req *core.Request) bool {
	logger := uip.GetLogger(req.Ctx)
	logger.Info("Saving session to the storage")
	err := uip.sessionStorage.Save(req.Ctx, req.Session)
	if err != nil {
		logger.Errorf("Session saving failed: %s", err)
		return false
	}
	return true
}

func (uip *UIPresenter) getOrCreateSession(ctx context.Context, chatID int) (*core.Session, error) {
	session, err := uip.sessionStorage.Get(ctx, chatID)
	if err != nil {
		return nil, errors.Wrap(err, "storage get")
	}
	logger := uip.GetLogger(ctx)
	if session == nil {
		logger.WithField("chat_id", chatID).Info("session doesn't exist for chat, init a new one")
		session = core.NewSession(chatID)
	} else {
		logger.WithField("session", session).Info("chat session from storage")
	}
	if session.LastPage == nil {
		session.LastPage = core.DefaultPageURL
	}
	return session, nil
}

func (uip *UIPresenter) sendError(ctx context.Context, msg *msgsqueue.Message) {
	logger := uip.GetLogger(ctx)
	logger.WithField("chat_id", msg.Chat.ID).Info("Sending error msg to the chat")
	_, err := uip.messenger.SendText(ctx, msg.Chat.ID, errorMessageText)
	if err != nil {
		logger.Errorf("Cannot send error msg: %s", err)
		return
	}
}

func (uip *UIPresenter) getPage(pageURL *core.URL) (page.Page, error) {
	pg, ok := uip.pageRegistry[pageURL.Page]
	if !ok {
		return nil, errors.Errorf("url %s leads to not known page %s", pageURL.Encode(), pageURL.Page)
	}
	return pg, nil
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
