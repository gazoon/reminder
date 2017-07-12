package models

import (
	"github.com/gazoon/bot_libs/queue/messages"
)

type Chat struct {
	msgsqueue.Chat
	Timezone int
}

type User struct {
	msgsqueue.User
}
