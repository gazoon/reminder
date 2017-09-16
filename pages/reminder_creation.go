package pages

import (
	"fmt"
	"github.com/gazoon/bot_libs/utils"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"reminder/core"
	"reminder/core/page"
	"reminder/models"
	"reminder/storages/chats"
	"reminder/storages/reminders"
	"time"
)

var (
	timeFormats = []string{"2006-01-02 15:04:05", "2006.01.02 15:04:05"}
)

type ReminderCreation struct {
	*page.BasePage

	Reminders reminders.Storage
	Chats     chats.Storage
}

func (rc *ReminderCreation) Init(builder *page.PagesBuilder) error {
	var err error
	controllers := map[string]page.Controller{
		"on_title":       rc.onTitleController,
		"on_date":        rc.onDateController,
		"on_description": rc.onDescriptionController,
		"done":           rc.doneController,
	}
	rc.BasePage, err = builder.NewBasePage("reminder_creation", nil, controllers)
	return err
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
	var remindAt time.Time
	for _, format := range timeFormats {
		var err error
		remindAt, err = time.ParseInLocation(format, req.Msg.Text, loc)
		if err == nil {
			break
		} else {
			fmt.Println(err)
		}

	}
	if remindAt.IsZero() {
		return page.BadInputResponse(fmt.Sprintf("cannot parse in any of these formats: %v", timeFormats))
	}
	rc.UpdateState(req, "remind_at", remindAt)
	rc.UpdateState(req, "last_enter", "date")
	return nil, nil, nil
}

func (rc *ReminderCreation) onDescriptionController(req *core.Request) (map[string]interface{}, *core.URL, error) {
	description := req.Msg.Text
	rc.UpdateState(req, "description", description)
	rc.UpdateState(req, "last_enter", "description")
	return nil, nil, nil
}

func (rc *ReminderCreation) doneController(req *core.Request) (map[string]interface{}, *core.URL, error) {
	data := rc.GetState(req)
	form := &ReminderForm{}
	err := mapstructure.Decode(data, form)
	if err != nil {
		return nil, nil, errors.Wrap(err, "to form decode")
	}
	err = utils.Validate.Struct(form)
	if err != nil {
		return nil, nil, errors.Wrap(err, "form validation")
	}
	reminder := models.NewReminder(req.Chat.ID, form.Title, form.RemindAt, form.Description)
	err = rc.Reminders.Save(req.Ctx, reminder)
	if err != nil {
		return nil, nil, errors.Wrap(err, "reminders storage save ")
	}
	return nil, nil, nil
}

type ReminderForm struct {
	Title       string    `mapstructure:"title" validate:"required"`
	RemindAt    time.Time `mapstructure:"remind_at" validate:"required"`
	Description *string   `mapstructure:"description"`
}
