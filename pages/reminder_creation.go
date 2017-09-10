package pages

import (
	"github.com/pkg/errors"
	"reminder/core"
	"reminder/core/page"
	"reminder/storages/chats"
	"reminder/storages/reminders"
	"time"
)

const (
	timeFormat = "2006-01-02 15:04:05"
)

type ReminderCreation struct {
	*page.BasePage

	Reminders reminders.Storage
	Chats     chats.Storage
}

func (rc *ReminderCreation) Init(builder *page.PagesBuilder) error {
	var err error
	controllers := map[string]page.Controller{
		"input_routing":  rc.inputRoutingController,
		"on_title":       rc.onTitleController,
		"on_date":        rc.onDateController,
		"on_description": rc.onDescriptionController,
		"done":           rc.doneController,
	}
	rc.BasePage, err = builder.NewBasePage("reminder_creation", nil, controllers)
	return err
}

func (rc *ReminderCreation) inputRoutingController(req *core.Request) (map[string]interface{}, *core.URL, error) {
	state := rc.GetState(req)
	if _, ok := state["last_enter"]; !ok {
		rc.UpdateState(req, "last_enter", "")
	}
	return nil, nil, nil
}

func (rc *ReminderCreation) onTitleController(req *core.Request) (map[string]interface{}, *core.URL, error) {
	title := req.Msg.Text
	rc.UpdateState(req, "title", title)
	rc.UpdateState(req, "last_enter", "title")
	return nil, nil, nil
}

func (rc *ReminderCreation) onDateController(req *core.Request) (map[string]interface{}, *core.URL, error) {
	chat, err := rc.Chats.Get(req.Ctx, req.Chat.ID)
	if err != nil {
		return nil, nil, errors.Wrap(err, "chats get failed")
	}
	if chat == nil {
		return map[string]interface{}{"no_timezone": true}, nil, nil
	}
	loc := chat.TimeLocation()
	remindAt, err := time.ParseInLocation(timeFormat, req.Msg.Text, loc)
	if err != nil {
		return page.BadInputResponse(err.Error())
	}
	rc.UpdateState(req, "remind_at", remindAt)
	rc.UpdateState(req, "last_enter", "date")
	return map[string]interface{}{"no_timezone": false, "error": false}, nil, nil
}

func (rc *ReminderCreation) onDescriptionController(req *core.Request) (map[string]interface{}, *core.URL, error) {
	description := req.Msg.Text
	rc.UpdateState(req, "description", description)
	rc.UpdateState(req, "last_enter", "description")
	return nil, nil, nil
}

func (rc *ReminderCreation) doneController(req *core.Request) (map[string]interface{}, *core.URL, error) {
	return nil, nil, nil
}
