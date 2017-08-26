package models

import (
	"time"

	"github.com/gazoon/bot_libs/queue/messages"
	"github.com/satori/go.uuid"
)

type Chat struct {
	msgsqueue.Chat
	Timezone int
}

type User struct {
	msgsqueue.User
}

type Reminder struct {
	ReminderID string
	ChatID     int
	Title      string
	RemindAt   time.Time
	CreatedAt  time.Time

	Description *string
}

func NewReminder(chatID int, title string, remindAt time.Time, description *string) *Reminder {
	return &Reminder{
		ReminderID:  uuid.NewV4().String(),
		ChatID:      chatID,
		Title:       title,
		RemindAt:    remindAt,
		CreatedAt:   time.Now(),
		Description: description,
	}
}
