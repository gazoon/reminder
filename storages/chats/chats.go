package chats

import (
	"context"
	"reminder/models"

	"github.com/gazoon/bot_libs/mongo"
	"github.com/pkg/errors"
	"gopkg.in/go-playground/validator.v9"
	"gopkg.in/mgo.v2/bson"

	"gopkg.in/mgo.v2"
)

var (
	validate *validator.Validate
)

func init() {
	validate = validator.New()
}

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
	ChatID    int    `bson:"chat_id"`
	Title     string `bson:"title"`
	Timezone  int    `bson:"timezone"`
	IsPrivate bool   `bson:"is_private"`
}

func DataFromModel(m *models.Chat) *Chat {
	return &Chat{
		ChatID:    m.ID,
		Title:     m.Title,
		Timezone:  m.Timezone,
		IsPrivate: m.IsPrivate,
	}
}

func (c *Chat) toModel() (*models.Chat, error) {
	err := validate.Struct(c)
	if err != nil {
		return nil, errors.Wrap(err, "bad data for chat")
	}
	return &models.Chat{
		ID:        c.ChatID,
		Title:     c.Title,
		Timezone:  c.Timezone,
		IsPrivate: c.IsPrivate,
	}, nil
}
