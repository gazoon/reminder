package main

import (
	"fmt"
	"reminder/core"

	"github.com/gazoon/bot_libs/logging"
	"github.com/gazoon/bot_libs/messenger"
)

func main() {
	fmt.Println("sdf")
	telegramMessenger, err := messenger.NewTelegram("441160175:AAHM00D7yNdxU9R5f4njLGEOs-zKBVZXbk4", 5)
	if err != nil {
		panic(err)
	}
	homePage, err := core.NewHomePage(telegramMessenger)
	if err != nil {
		panic(err)
	}
	listPage, err := core.NewReminderListPage(telegramMessenger)
	if err != nil {
		panic(err)
	}
	a1 := homePage.MainPart
	a2 := listPage.MainPart
	b1 := homePage.OtherParts
	b2 := listPage.OtherParts
	h := listPage.InputHandlers
	fmt.Println(logging.ObjToString(a1))
	fmt.Println(logging.ObjToString(a2))
	fmt.Println(logging.ObjToString(b1))
	fmt.Println(logging.ObjToString(b2))
	fmt.Println(h)
}
