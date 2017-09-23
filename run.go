package main

import (
	"reminder/env"

	"flag"
	"reminder/config"

	"github.com/gazoon/bot_libs/gateway"
	"github.com/gazoon/bot_libs/logging"
	"github.com/gazoon/bot_libs/queue/messages"
	"github.com/gazoon/bot_libs/utils"
	"github.com/pkg/errors"
	"reminder/reminders_sender"
)

var (
	gLogger = logging.WithPackage("run")
)

func main() {
	var confPath string
	config.FromCmdArgs(&confPath)
	flag.Parse()

	env.Initialization(confPath)
	conf := config.GetInstance()
	incomingQueue, err := env.CreateMongoMsgs()
	if err != nil {
		panic(err)
	}
	telegramMessenger, err := env.CreateTelegramMessenger()
	if err != nil {
		panic(err)
	}
	pollerService := gateway.NewTelegramPoller(incomingQueue, conf.Telegram.APIToken, conf.Telegram.BotName,
		conf.TelegramPolling.PollTimeout, conf.TelegramPolling.RetryDelay)
	remindersStorage, err := env.CreateMongoRemindersStorage()
	if err != nil {
		panic(err)
	}
	chatsStorage, err := env.CreateMongoChatsStorage()
	if err != nil {
		panic(err)
	}
	presenter, err := env.CreateUIPresenter(telegramMessenger, remindersStorage, chatsStorage)
	if err != nil {
		panic(err)
	}
	readerService := msgsqueue.NewReader(incomingQueue, conf.MongoMessages.WorkersNum, presenter.OnQueueMessage)
	remindersSenderService := remsender.NewSender(presenter, remindersStorage, conf.MongoReminders.WorkersNum)
	gLogger.Info("Starting bot service")
	readerService.Start()
	defer readerService.Stop()
	gLogger.Info("Starting telegram poller service")
	err = pollerService.Start()
	if err != nil {
		panic(errors.Wrap(err, "cannot start poller"))
	}
	//remindersSenderService.Start()
	defer remindersSenderService.Stop()
	gLogger.Info("Server successfully started")
	utils.WaitingForShutdown()
}
