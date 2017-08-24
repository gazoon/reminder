package models

import (
	"github.com/gazoon/bot_libs/queue/messages"
	"time"
)

type Chat struct {
	msgsqueue.Chat
	Timezone int
}

type User struct {
	msgsqueue.User
}

type Reminder struct {
	ReminderID  string
	ChatID      int
	Title       string
	RemindAt        time.Time
	CreatedAt time.Time

	Description *string
}
