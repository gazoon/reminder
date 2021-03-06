package env

import (
	"reminder/config"
	"reminder/core"
	"reminder/core/page"
	"reminder/core/presenter"

	"github.com/gazoon/bot_libs/logging"
	"github.com/gazoon/bot_libs/messenger"
	"github.com/gazoon/bot_libs/queue/messages"
	"github.com/pkg/errors"
	"reminder/pages"
	"reminder/storages/chats"
	"reminder/storages/reminders"
)

const (
	pageViewsFolder = "views"
)

var (
	gLogger = logging.WithPackage("env")
)

func Initialization(confPath string) {
	config.Initialization(confPath)
	conf := config.GetInstance()
	logging.PatchStdLog(conf.Logging.DefaultLevel, conf.ServiceName, conf.ServerID)
	gLogger.Info("Environment has been initialized")
}

func CreateTelegramMessenger() (messenger.Messenger, error) {
	conf := config.GetInstance()
	telegramMessenger, err := messenger.NewTelegram(conf.Telegram.APIToken, conf.Telegram.HttpTimeout)
	return telegramMessenger, errors.Wrap(err, "telegram messenger")
}

func CreateMongoMsgs() (*msgsqueue.MongoQueue, error) {
	conf := config.GetInstance().MongoMessages
	incomingMongoQueue, err := msgsqueue.NewMongoQueue(conf.Database, conf.Collection, conf.User, conf.Password, conf.Host,
		conf.Port, conf.Timeout, conf.PoolSize, conf.RetriesNum, conf.RetriesInterval, conf.FetchDelay)
	return incomingMongoQueue, errors.Wrap(err, "mongo messages queue")
}

func CreateMongoRemindersStorage() (*reminders.MongoStorage, error) {
	conf := config.GetInstance().MongoReminders
	storage, err := reminders.NewMongoStorage(conf.Database, conf.Collection, conf.User, conf.Password, conf.Host,
		conf.Port, conf.Timeout, conf.PoolSize, conf.RetriesNum, conf.RetriesInterval, conf.FetchDelay)
	return storage, errors.Wrap(err, "mongo reminders storage")
}

func CreateMongoChatsStorage() (chats.Storage, error) {
	conf := config.GetInstance().MongoChats
	storage, err := chats.NewMongoStorage(conf.Database, conf.Collection, conf.User, conf.Password, conf.Host,
		conf.Port, conf.Timeout, conf.PoolSize, conf.RetriesNum, conf.RetriesInterval)
	return storage, errors.Wrap(err, "mongo chats storage")
}

func CreateUIPresenter(messenger messenger.Messenger, remindersStorage reminders.Storage, chatsStorage chats.Storage) (
	*presenter.UIPresenter, error) {

	builder := page.NewPagesBuilder(messenger, pageViewsFolder)
	pagesRegistry, err := builder.InstantiatePages(
		&pages.ChangeTimezone{Chats: chatsStorage},
		&pages.Home{},
		&pages.NotFound{},
		&pages.ReminderList{Reminders: remindersStorage},
		&pages.ShowReminder{Reminders: remindersStorage, Chats: chatsStorage},
		&pages.ReminderCreation{Reminders: remindersStorage, Chats: chatsStorage, Messenger: messenger},
	)
	if err != nil {
		return nil, errors.Wrap(err, "pages registry")
	}
	conf := config.GetInstance().MongoSessions
	sessionStorage, err := core.NewMongoStorage(conf.Database, conf.Collection, conf.User, conf.Password, conf.Host,
		conf.Port, conf.Timeout, conf.PoolSize, conf.RetriesNum, conf.RetriesInterval)
	if err != nil {
		return nil, errors.Wrap(err, "mongo storage")
	}
	return presenter.New(messenger, sessionStorage, pagesRegistry, nil), nil
}
