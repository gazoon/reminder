package pages

import (
	"reminder/core"
	"reminder/core/page"
	"reminder/reminders"

	"fmt"
	"github.com/pkg/errors"
	"reminder/models"
	"time"
)

type ShowReminder struct {
	*page.BasePage

	Storage reminders.Storage
}

func (sr *ShowReminder) Init(builder *page.PagesBuilder) error {
	var err error
	sr.BasePage, err = builder.NewBasePage("show_reminder", sr.globalController, nil)
	return err
}

func (sr *ShowReminder) globalController(req *core.Request) (map[string]interface{}, *core.URL, error) {
	reminderID := req.URL.Params["reminder_id"]
	if reminderID == "" {
		return nil, nil, errors.New("'reminder_id' not found in url params")
	}
	reminder, err := sr.Storage.Get(req.Ctx, reminderID)
	if err != nil {
		return nil, nil, errors.Wrap(err, "reminders storage get failed")
	}
	data := map[string]interface{}{
		"title":       reminder.Title,
		"created_at":  reminder.CreatedAt,
		"remind_at":   reminder.RemindAt,
		"description": "",
	}
	loc := time.FixedZone("", models.DefaultTimezone)
	fmt.Println(reminder.RemindAt.UTC())
	fmt.Println(reminder.RemindAt)
	fmt.Println(reminder.RemindAt.In(loc))
	loc = time.FixedZone("", models.DefaultTimezone+3600)
	fmt.Println(reminder.RemindAt.In(loc))
	if reminder.Description != nil {
		data["description"] = *reminder.Description
	}
	return data, nil, nil
}
