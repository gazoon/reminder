package pages

import (
	"github.com/pkg/errors"
	"reminder/core"
	"reminder/core/page"
	"reminder/models"
	"reminder/storages/chats"
	"strconv"
)

type ChangeTimezone struct {
	*page.BasePage

	Chats chats.Storage
}

func (ct *ChangeTimezone) Init(builder *page.PagesBuilder) error {
	controllers := map[string]page.Controller{
		"on_timezone": ct.onTimezoneController,
	}
	var err error
	ct.BasePage, err = builder.NewBasePage("change_timezone", nil, controllers)
	return err
}

func (ct *ChangeTimezone) onTimezoneController(req *core.Request) (map[string]interface{}, *core.URL, error) {
	ct.GetLogger(req.Ctx).Infof("on timezone input: %s", req.MsgText)
	timezoneInMinutes, err := strconv.Atoi(req.MsgText)
	if err != nil {
		return page.BadInputResponse(err.Error())
	}
	timezoneInSeconds := timezoneInMinutes * 60
	chat, err := ct.Chats.Get(req.Ctx, req.ChatID)
	if err != nil {
		return nil, nil, errors.Wrap(err, "chats storage get")
	}
	if chat == nil {
		chat = models.NewChat(req.ChatID, timezoneInSeconds)
	} else {
		chat.Timezone = timezoneInSeconds
	}
	err = ct.Chats.Save(req.Ctx, chat)
	if err != nil {
		return nil, nil, errors.Wrap(err, "chats storage save")
	}
	return nil, nil, nil
}
