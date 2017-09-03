package pages

import (
	"reminder/core"
	"reminder/core/page"
	"time"
	"reminder/reminders"
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
	data := map[string]interface{}{
		"title":       "foo",
		"created_at":        time.Now(),
		"remind_at":        time.Now().UTC(),
		"description": "ssssssss",
	}
	return data, nil, nil
}
