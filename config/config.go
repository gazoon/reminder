package config

import (
	"sync"

	"github.com/gazoon/bot_libs/config"
)

var (
	once     sync.Once
	instance *ServiceConfig
)

type ServiceConfig struct {
	config.BaseConfig `mapstructure:",squash" json:",inline"`
	MongoMessages     *config.MongoQueue       `mapstructure:"mongo_messages" json:"mongo_messages"`
	MongoStorage      *config.MongoDBSettings  `mapstructure:"mongo_storage" json:"mongo_storage"`
	Telegram          *config.TelegramSettings `mapstructure:"telegram" json:"telegram"`
	TelegramPolling   *config.TelegramPolling  `mapstructure:"telegram_polling" json:"telegram_polling"`
	Logging           *config.Logging          `mapstructure:"logging" json:"logging"`
}

func Initialization(configPath string) {
	once.Do(func() {
		instance = &ServiceConfig{}
		err := config.FromJSONFile(configPath, instance)
		if err != nil {
			panic(err)
		}
	})
}

func GetInstance() *ServiceConfig {
	return instance
}

func FromCmdArgs(confPath *string) {
	config.FromCmdArgs(confPath)
}
