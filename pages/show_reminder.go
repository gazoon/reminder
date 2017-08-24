package pages

import (
	"reminder/core"
	"reminder/core/page"
	"time"
)

type ShowReminderPage struct {
	*page.BasePage
}

func NewShowReminder(builder *page.PagesBuilder) (page.Page, error) {
	p := new(ShowReminderPage)
	var err error
	p.BasePage, err = builder.NewBasePage("show_reminder", p.globalController, nil)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (sr *ShowReminderPage) globalController(req *core.Request) (map[string]interface{}, *core.URL, error) {
	data := map[string]interface{}{
		"title":       "foo",
		"date":        time.Now(),
		"description": "ssssssss",
	}
	return data, nil, nil
}
