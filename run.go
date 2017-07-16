package main

import (
	"reminder/env"

	"flag"
	"reminder/config"

	"github.com/gazoon/bot_libs/gateway"
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
}
