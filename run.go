package main

import (
	"fmt"
	"reminder/core"

	"context"
	"reminder/core/pages"
	"reminder/models"

	log "github.com/Sirupsen/logrus"
	"github.com/gazoon/bot_libs/logging"
	"github.com/gazoon/bot_libs/messenger"
	"github.com/gazoon/bot_libs/queue/messages"
)

func main() {
	telegramMessenger, err := messenger.NewTelegram("282857391:AAEEdYoGCEa-MzLMKXUABvSTaOLUSaSS53Y", 5)
	if err != nil {
		panic(err)
	}
	//homePage, err := pages.NewHomePage(telegramMessenger)
	//if err != nil {
	//	panic(err)
	//}
	listPage, err := pages.NewReminderListPage(telegramMessenger)
	if err != nil {
		panic(err)
	}
	session := new(core.Session)
	msg := &msgsqueue.Message{Chat: &msgsqueue.Chat{ID: 231193206}, From: &msgsqueue.User{Name: "ooooo"}}
	chat := &models.Chat{Chat: *msg.Chat}
	user := &models.User{User: *msg.From}
	req := &core.Request{Ctx: logging.NewContext(context.Background(), log.WithField("fooo", "bar")), Msg: msg, Chat: chat, Session: session, User: user}
	req.URL = core.NewURL("ooo", "has_reminders", nil)
	uri, err := listPage.Enter(req)
	fmt.Println(uri, err)
	fmt.Println(listPage.Intents)
	fmt.Println(req.Session.LocalIntents)
}
