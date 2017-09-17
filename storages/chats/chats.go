package chats

import (
	"context"
	"reminder/models"

	"github.com/gazoon/bot_libs/mongo"
	"github.com/gazoon/bot_libs/utils"
	"github.com/pkg/errors"
	"gopkg.in/mgo.v2/bson"

	"gopkg.in/mgo.v2"
)

type Storage interface {
	Get(ctx context.Context, ChatID int) (*models.Chat, error)
	Save(ctx context.Context, chat *models.Chat) error
}

type MongoStorage struct {
	client *mongo.Client
}

func NewMongoStorage(database, collection, user, password, host string, port, timeout, poolSize, retriesNum,
	retriesInterval int) (*MongoStorage, error) {

	client, err := mongo.NewClient(database, collection, user, password, host, port, timeout, poolSize, retriesNum,
		retriesInterval)
	if err != nil {
		return nil, err
	}
	return &MongoStorage{client: client}, nil
}

func (ms *MongoStorage) Get(ctx context.Context, chatID int) (*models.Chat, error) {
	data := &Chat{}
	err := ms.client.FindOne(ctx, bson.M{"chat_id": chatID}, data)
	if err == mgo.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "mongo find one")
	}
	return data.toModel()
}

func (ms *MongoStorage) Save(ctx context.Context, chat *models.Chat) error {
	data := DataFromModel(chat)
	err := ms.client.UpsertRetry(ctx, bson.M{"chat_id": chat.ID}, data)
	return errors.Wrap(err, "mongo upsert")
}

type Chat struct {
	ChatID   int `bson:"chat_id"`
	Timezone int `bson:"timezone"`
}

func DataFromModel(m *models.Chat) *Chat {
	return &Chat{
		ChatID:   m.ID,
		Timezone: m.Timezone,
	}
}

func (c *Chat) toModel() (*models.Chat, error) {
	err := utils.Validate.Struct(c)
	if err != nil {
		return nil, errors.Wrap(err, "bad data for chat")
	}
	return &models.Chat{
		ID:       c.ChatID,
		Timezone: c.Timezone,
	}, nil
}
