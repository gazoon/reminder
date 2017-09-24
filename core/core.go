package core

import (
	"context"
	"net/url"
	"strings"

	"github.com/gazoon/bot_libs/logging"
	"github.com/gazoon/bot_libs/mongo"
	"github.com/gazoon/bot_libs/queue/messages"
	"github.com/pkg/errors"
	"github.com/satori/go.uuid"
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
)

const (
	urlScheme = "page"
)

var (
	gLogger         = logging.WithPackage("core")
	HomePageURL     = NewURL("home", "", nil)
	NotFoundPageURL = NewURL("not_found", "", nil)
	DefaultPageURL  = HomePageURL
)

type URL struct {
	Page   string
	Action string
	Params map[string]string
}

func NewURL(page, action string, params map[string]string) *URL {
	if params == nil {
		params = make(map[string]string)
	}
	return &URL{Page: page, Action: action, Params: params}
}

func NewURLFromStr(rawurl string) (*URL, error) {
	if rawurl == "" {
		return nil, errors.New("empty url string")
	}
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, errors.Wrap(err, "url parsing")
	}
	if u.Scheme != urlScheme && u.Scheme != "" {
		return nil, errors.Errorf("supported scheme %s, found %s", urlScheme, u.Scheme)
	}
	queryValues := u.Query()
	params := make(map[string]string, len(queryValues))
	for k := range queryValues {
		params[k] = queryValues.Get(k)
	}
	action := strings.Trim(u.Path, "/")
	return NewURL(u.Host, action, params), nil
}

func (u *URL) Encode() string {
	if u == nil {
		return ""
	}
	queryValues := make(url.Values, len(u.Params))
	for k, v := range u.Params {
		queryValues.Set(k, v)
	}
	underlingURL := url.URL{Host: u.Page, Path: u.Action, Scheme: urlScheme, RawQuery: queryValues.Encode()}
	return underlingURL.String()
}

func (u URL) String() string {
	return logging.ObjToString(&u)
}

func (u *URL) IsRelative() bool {
	return u.Page == ""
}

func (u *URL) Copy() *URL {
	return NewURL(u.Page, u.Action, u.Params)
}

type Message interface {
}

type TextMessage struct {
	Text string
}

type Request struct {
	Session        *Session
	Ctx            context.Context
	Msg            Message
	MsgID          int
	MsgText        string
	ChatID         int
	URL            *URL
	Intents        []*Intent
	SaveSentMsgIDs bool
}

func NewRequestFromQueueMsg(ctx context.Context, queueMsg *msgsqueue.Message) *Request {
	msgText := queueMsg.Text
	reqURL, err := NewURLFromStr(msgText)
	if err != nil {
		reqURL = nil
	} else if reqURL.IsRelative() {
		reqURL = nil
	}
	msg := &TextMessage{Text: msgText}
	return &Request{Ctx: ctx, MsgText: msgText, Msg: msg, MsgID: queueMsg.MessageID, ChatID: queueMsg.Chat.ID, URL: reqURL}
}

func (r *Request) SetSession(s *Session) {
	r.Session = s
	r.Intents = s.LocalIntents
}

type Session struct {
	ID           string
	ChatID       int
	LocalIntents []*Intent
	LastPage     *URL
	InputHandler *URL
	PagesStates  map[string]map[string]interface{}
	GlobalState  map[string]interface{}
}

func NewSession(chatID int) *Session {
	sessionID := uuid.NewV4().String()
	globalState := make(map[string]interface{})
	pagesState := make(map[string]map[string]interface{})
	return &Session{ChatID: chatID, ID: sessionID, GlobalState: globalState, PagesStates: pagesState}
}

func (s Session) String() string {
	return logging.ObjToString(&s)
}

func (s *Session) AddIntent(words []string, handler *URL) {
	s.LocalIntents = append(s.LocalIntents, NewIntent(handler, words))
}

func (s *Session) SetInputHandler(ctx context.Context, handler *URL) {
	logger := logging.FromContextAndBase(ctx, gLogger)
	logger.Infof("Change input handler new: %s, old: %s", handler, s.InputHandler)
	s.InputHandler = handler
}

func (s *Session) ResetInputHandler(ctx context.Context) {
	logger := logging.FromContextAndBase(ctx, gLogger)
	logger.Infof("Reset input handler %s", s.InputHandler.Encode())
	s.InputHandler = nil
}

func (s *Session) ResetIntents(ctx context.Context) {
	logger := logging.FromContextAndBase(ctx, gLogger)
	logger.Infof("Reset local intents %v", s.LocalIntents)
	s.LocalIntents = nil
}

func (s *Session) SetLastPage(ctx context.Context, newLastPage *URL) {
	logger := logging.FromContextAndBase(ctx, gLogger)
	logger.Infof("Change last page %s ---> %s", s.LastPage.Encode(), newLastPage.Encode())
	s.LastPage = newLastPage
}

type Storage interface {
	Get(ctx context.Context, chatID int) (*Session, error)
	Save(ctx context.Context, session *Session) error
	Delete(ctx context.Context, session *Session) error
}

//type InMemoryStorage struct {
//	mx      sync.RWMutex
//	storage map[int][]byte
//}
//
//func (ms *InMemoryStorage) Get(ctx context.Context, chatID int) (*Session, error) {
//	ms.mx.RLock()
//	sessionData, ok := ms.storage[chatID]
//	ms.mx.RUnlock()
//	if !ok {
//		return nil, nil
//	}
//	session := &Session{}
//	err := json.Unmarshal(sessionData, session)
//	if err != nil {
//		return nil, errors.Wrap(err, "session data unmarshal")
//	}
//	return session, nil
//}
//
//func (ms *InMemoryStorage) Save(ctx context.Context, session *Session) error {
//	sessionData, err := json.Marshal(session)
//	if err != nil {
//		return errors.Wrap(err, "session marshal")
//	}
//	ms.mx.Lock()
//	ms.storage[session.ChatID] = sessionData
//	ms.mx.Unlock()
//	return nil
//}
//
//func (ms *InMemoryStorage) Delete(ctx context.Context, session *Session) error {
//	ms.mx.Lock()
//	defer ms.mx.Unlock()
//	delete(ms.storage, session.ChatID)
//	return nil
//}

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

func (ms *MongoStorage) Get(ctx context.Context, chatID int) (*Session, error) {
	data := &SessionInMongo{}
	err := ms.client.FindOne(ctx, bson.M{"chat_id": chatID}, data)
	if err == mgo.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "mongo find one")
	}
	session, err := data.ToSession()
	return session, errors.Wrapf(err, "cannot build session from mongo representation %s", data)
}

func (ms *MongoStorage) Save(ctx context.Context, session *Session) error {
	sessionData := NewSessionInMongo(session)
	err := ms.client.UpsertRetry(ctx, bson.M{"chat_id": session.ChatID}, sessionData)
	return errors.Wrap(err, "mongo upsert")
}

func (ms *MongoStorage) Delete(ctx context.Context, session *Session) error {
	_, err := ms.client.Remove(ctx, bson.M{"chat_id": session.ChatID})
	return errors.Wrap(err, "mongo remove")
}

type IntentInMongo struct {
	Handler string   `bson:"handler"`
	Words   []string `bson:"words"`
}

type SessionInMongo struct {
	SessionID    string                            `bson:"session_id"`
	ChatID       int                               `bson:"chat_id"`
	LocalIntents []*IntentInMongo                  `bson:"local_intents"`
	LastPage     string                            `bson:"last_page"`
	InputHandler string                            `bson:"input_handler"`
	PagesStates  map[string]map[string]interface{} `bson:"pages_states"`
	GlobalState  map[string]interface{}            `bson:"global_states"`
}

func NewSessionInMongo(session *Session) *SessionInMongo {
	sm := new(SessionInMongo)
	sm.SessionID = session.ID
	sm.ChatID = session.ChatID
	sm.InputHandler = session.InputHandler.Encode()
	sm.LastPage = session.LastPage.Encode()
	sm.GlobalState = session.GlobalState
	sm.PagesStates = session.PagesStates
	sm.LocalIntents = make([]*IntentInMongo, len(session.LocalIntents))
	for i, intent := range session.LocalIntents {
		sm.LocalIntents[i] = &IntentInMongo{intent.Handler.Encode(), intent.Words}
	}
	return sm
}

func (sm SessionInMongo) String() string {
	return logging.ObjToString(&sm)
}

func (sm *SessionInMongo) ToSession() (*Session, error) {
	if sm.ChatID == 0 {
		return nil, errors.New("chat_id field doesn't present")
	}
	if sm.SessionID == "" {
		return nil, errors.New("session_id field doesn't present")
	}
	model := &Session{ID: sm.SessionID, ChatID: sm.ChatID, PagesStates: sm.PagesStates, GlobalState: sm.GlobalState}
	localIntents := make([]*Intent, len(sm.LocalIntents))
	for i, intentData := range sm.LocalIntents {
		intent, err := NewIntentStrHandler(intentData.Handler, intentData.Words)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot build local intent %d from storage data", i)
		}
		localIntents[i] = intent
	}
	var inputHandlerURL *URL
	if sm.InputHandler != "" {
		var err error
		inputHandlerURL, err = NewURLFromStr(sm.InputHandler)
		if err != nil {
			return nil, errors.Wrap(err, "storage contains bad input handler")
		}
	}
	lastPageURL, err := NewURLFromStr(sm.LastPage)
	if err != nil {
		return nil, errors.Wrap(err, "storage contains bad last page")
	}
	model.LocalIntents = localIntents
	model.InputHandler = inputHandlerURL
	model.LastPage = lastPageURL
	return model, nil
}
