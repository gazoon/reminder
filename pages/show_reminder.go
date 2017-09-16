package pages

import (
	"reminder/core"
	"reminder/core/page"
	"reminder/storages/chats"
	"reminder/storages/reminders"

	"github.com/pkg/errors"
)

type ShowReminder struct {
	*page.BasePage

	Reminders reminders.Storage
	Chats     chats.Storage
}

func (sr *ShowReminder) Init(builder *page.PagesBuilder) error {
	var err error
	sr.BasePage, err = builder.NewBasePage("show_reminder", sr.globalController, nil)
	return err
}

func (sr *ShowReminder) globalController(req *core.Request) (map[string]interface{}, *core.URL, error) {
	chat, err := sr.Chats.Get(req.Ctx, req.Chat.ID)
	if err != nil {
		return nil, nil, errors.Wrap(err, "chats storage get")
	}
	reminderID := req.URL.Params["reminder_id"]
	if reminderID == "" {
		return nil, nil, errors.New("'reminder_id' not found in url params")
	}
	reminder, err := sr.Reminders.Get(req.Ctx, reminderID)
	if err != nil {
		return nil, nil, errors.Wrap(err, "reminders storage get failed")
	}
	if reminder == nil {
		return map[string]interface{}{"reminder_not_found": true}, nil, nil
	}

	data := map[string]interface{}{"title": reminder.Title}
	if reminder.Description != nil {
		data["description"] = *reminder.Description
	} else {
		data["description"] = ""
	}
	if chat == nil {
		data["created_at"] = reminder.CreatedAt
		data["remind_at"] = reminder.RemindAt
	} else {
		data["created_at"] = reminder.CreatedAtLocal(chat)
		data["remind_at"] = reminder.RemindAtLocal(chat)
	}
	return data, nil, nil
}
