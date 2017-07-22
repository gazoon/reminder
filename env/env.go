package env

import (
	"github.com/gazoon/bot_libs/logging"
	"github.com/gazoon/bot_libs/messenger"
	"github.com/gazoon/bot_libs/queue/messages"
	"github.com/pkg/errors"
	"reminder/config"
	"reminder/core"
	"reminder/core/pages"
	"reminder/presenter"
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
	incomingMongoQueue, err := msgsqueue.NewMongoQueue(conf.Database, conf.User, conf.Password, conf.Host, conf.Port,
		conf.Timeout, conf.PoolSize, conf.RetriesNum, conf.RetriesInterval, conf.FetchDelay)
	return incomingMongoQueue, errors.Wrap(err, "mongo messages queue")
}

func CreateUIPresenter(messenger messenger.Messenger) (*presenter.UIPresenter, error) {
	pagesRegistry, err := pages.GetRegisteredPages(messenger)
	if err != nil {
		return nil, errors.Wrap(err, "pages registry")
	}
	conf := config.GetInstance().MongoStorage
	sessionStorage, err := core.NewMongoStorage(conf.Database, conf.User, conf.Password, conf.Host, conf.Port, conf.Timeout,
		conf.PoolSize, conf.RetriesNum, conf.RetriesInterval)
	if err != nil {
		return nil, errors.Wrap(err, "mongo storage")
	}
	return presenter.New(messenger, sessionStorage, pagesRegistry, nil), nil
}
