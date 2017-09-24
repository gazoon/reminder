package models

import (
	"time"

	"github.com/gazoon/bot_libs/logging"
	"github.com/satori/go.uuid"
)

const (
	DefaultTimezone = 10800
)

type Chat struct {
	ID       int
	Timezone int
}

func NewChat(chatID, timezone int) *Chat {
	return &Chat{ID: chatID, Timezone: timezone}
}

func (c *Chat) timeDelta() time.Duration {
	return time.Hour * time.Duration(c.Timezone)
}

func (c *Chat) ToLocalTime(t time.Time) time.Time {
	return t.Add(c.timeDelta())
}

func (c *Chat) ToUTC(t time.Time) time.Time {
	return t.Add(-c.timeDelta())
}

type Reminder struct {
	ID        string
	ChatID    int
	Title     string
	RemindAt  time.Time
	CreatedAt time.Time

	Description *string
}

func NewReminder(chatID int, title string, remindAt time.Time, description *string) *Reminder {
	return &Reminder{
		ID:          uuid.NewV4().String(),
		ChatID:      chatID,
		Title:       title,
		RemindAt:    remindAt,
		CreatedAt:   time.Now().UTC(),
		Description: description,
	}
}

func (r *Reminder) RemindAtLocal(chat *Chat) time.Time {
	return chat.ToLocalTime(r.RemindAt)
}

func (r *Reminder) CreatedAtLocal(chat *Chat) time.Time {
	return chat.ToLocalTime(r.CreatedAt)
}

func (r Reminder) String() string {
	return logging.ObjToString(&r)
}
