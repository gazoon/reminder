package models

import (
	"time"

	"github.com/satori/go.uuid"
)

const (
	DefaultTimezone = 10800
)

type Chat struct {
	ID        int
	Title     string
	IsPrivate bool
	Timezone  int
}

func NewChat(chatID, timezone int, title string, isPrivate bool) *Chat {
	return &Chat{ID: chatID, Timezone: timezone, Title: title, IsPrivate: isPrivate}
}

func (c *Chat) ToLocalTime(t time.Time) time.Time {
	loc := time.FixedZone("", c.Timezone)
	return t.In(loc)
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
		CreatedAt:   time.Now(),
		Description: description,
	}
}

func (r *Reminder) RemindAtLocal(chat *Chat) time.Time {
	return chat.ToLocalTime(r.RemindAt)
}

func (r *Reminder) CreatedAtLocal(chat *Chat) time.Time {
	return chat.ToLocalTime(r.CreatedAt)
}
