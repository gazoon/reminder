package pages

import (
	"reminder/core"
	"reminder/core/page"
	"reminder/storages/chats"
	"reminder/storages/reminders"

	"reminder/models"

	"github.com/pkg/errors"
)

type ReminderReadyMessage struct {
	Reminder *models.Reminder
}

type ShowReminder struct {
	*page.BasePage

	Reminders reminders.Storage
	Chats     chats.Storage
}

func (sr *ShowReminder) Init(builder *page.PagesBuilder) error {
	var err error
	controllers := map[string]page.Controller{
		"show":       sr.showController,
		"when_ready": sr.whenReadyController,
	}
	sr.BasePage, err = builder.NewBasePage("show_reminder", nil, controllers)
	return err
}

func (sr *ShowReminder) whenReadyController(req *core.Request) (map[string]interface{}, *core.URL, error) {
	msg, ok := req.Msg.(*ReminderReadyMessage)
	if !ok {
		return nil, nil, errors.Errorf("expected ReminderReadyMessage, got: %v", req.Msg)
	}
	chat, err := sr.Chats.Get(req.Ctx, req.ChatID)
	if err != nil {
		return nil, nil, errors.Wrap(err, "chats storage get")
	}
	return reminderToData(msg.Reminder, chat), nil, nil
}

func (sr *ShowReminder) showController(req *core.Request) (map[string]interface{}, *core.URL, error) {
	chat, err := sr.Chats.Get(req.Ctx, req.ChatID)
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
	return reminderToData(reminder, chat), nil, nil
}

func reminderToData(reminder *models.Reminder, chat *models.Chat) map[string]interface{} {
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
	return data
}
