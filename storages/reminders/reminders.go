package reminders

import (
	"context"
	"reminder/models"

	"github.com/gazoon/bot_libs/logging"
	"github.com/gazoon/bot_libs/mongo"
	"github.com/gazoon/bot_libs/queue"
	"github.com/gazoon/bot_libs/utils"
	"github.com/pkg/errors"
	"gopkg.in/mgo.v2/bson"

	"time"

	"gopkg.in/mgo.v2"
)

type Storage interface {
	List(ctx context.Context, chatID int) ([]*models.Reminder, error)
	Get(ctx context.Context, reminderID string) (*models.Reminder, error)
	Delete(ctx context.Context, reminderID string) error
	Save(ctx context.Context, reminder *models.Reminder) error
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

func (ms *MongoStorage) List(ctx context.Context, chatID int) ([]*models.Reminder, error) {
	data := []*Reminder{}
	err := ms.client.Find(ctx, bson.M{"chat_id": chatID}, "created_at", -1, -1, &data)
	if err != nil {
		return nil, errors.Wrap(err, "mongo find")
	}
	reminders := make([]*models.Reminder, len(data))
	for i, reminderData := range data {
		var err error
		reminders[i], err = reminderData.toModel()
		if err != nil {
			return nil, err
		}
	}
	return reminders, nil
}

func (ms *MongoStorage) Get(ctx context.Context, reminderID string) (*models.Reminder, error) {
	data := &Reminder{}
	err := ms.client.FindOne(ctx, bson.M{"reminder_id": reminderID}, data)
	if err == mgo.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "mongo find one")
	}
	return data.toModel()
}

func (ms *MongoStorage) Delete(ctx context.Context, reminderID string) error {
	_, err := ms.client.Remove(ctx, bson.M{"reminder_id": reminderID})
	return errors.Wrap(err, "mongo remove")
}

func (ms *MongoStorage) Save(ctx context.Context, reminder *models.Reminder) error {
	data := DataFromModel(reminder)
	err := ms.client.UpsertRetry(ctx, bson.M{"reminder_id": reminder.ID}, data)
	return errors.Wrap(err, "mongo upsert")
}

type Reminder struct {
	ReminderID string    `bson:"reminder_id"`
	ChatID     int       `bson:"chat_id"`
	Title      string    `bson:"title"`
	RemindAt   time.Time `bson:"remind_at"`
	CreatedAt  time.Time `bson:"created_at"`

	Description *string `bson:"description"`
}

func DataFromModel(m *models.Reminder) *Reminder {
	return &Reminder{
		ReminderID:  m.ID,
		ChatID:      m.ChatID,
		Title:       m.Title,
		RemindAt:    m.RemindAt,
		CreatedAt:   m.CreatedAt,
		Description: m.Description,
	}
}

func (r *Reminder) toModel() (*models.Reminder, error) {
	err := utils.Validate.Struct(r)
	if err != nil {
		return nil, errors.Wrap(err, "bad data for reminder")
	}
	return &models.Reminder{
		ID:          r.ReminderID,
		ChatID:      r.ChatID,
		Title:       r.Title,
		RemindAt:    r.RemindAt,
		CreatedAt:   r.CreatedAt,
		Description: r.Description,
	}, nil
}

var (
	gLogger = logging.WithPackage("notification_queue")
)

type Reader interface {
	GetNext() (*models.Reminder, bool)
	StopGivingMsgs()
}

type MongoReader struct {
	client *mongo.Client
	*queue.BaseConsumer
}

func NewMongoReader(database, collection, user, password, host string, port, timeout, poolSize, retriesNum, retriesInterval,
	fetchDelay int) (*MongoReader, error) {

	client, err := mongo.NewClient(database, collection, user, password, host, port, timeout, poolSize, retriesNum,
		retriesInterval)
	if err != nil {
		return nil, err
	}
	return &MongoReader{client: client, BaseConsumer: queue.NewBaseConsumer(fetchDelay)}, nil
}

func (mq *MongoReader) GetNext() (*models.Reminder, bool) {
	var reminder *models.Reminder
	isStopped := mq.FetchLoop(func() bool {
		var isFetched bool
		reminder, isFetched = mq.tryGetNext()
		return isFetched
	})
	return reminder, isStopped
}

func (mq *MongoReader) tryGetNext() (*models.Reminder, bool) {
	result := &Reminder{}
	err := mq.client.FindAndModify(context.Background(),
		bson.M{"remind_at": bson.M{"$lt": time.Now()}},
		"remind_at",
		mgo.Change{Remove: true},
		result)
	if err != nil {
		if err != mgo.ErrNotFound {
			gLogger.Errorf("Cannot fetch document from mongo: %s", err)
		}
		return nil, false
	}
	reminder, err := result.toModel()
	if err != nil {
		gLogger.Errorf("Fetched document with bad reminder data: %s", err)
		return nil, false
	}
	return reminder, true
}
