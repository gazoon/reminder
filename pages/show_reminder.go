package pages

import (
	"reminder/core"
	"reminder/core/page"
	"time"
)

type ShowReminder struct {
	*page.BasePage
}

func (sr *ShowReminder) Init(builder *page.PagesBuilder) error {
	var err error
	sr.BasePage, err = builder.NewBasePage("show_reminder", sr.globalController, nil)
	return err
}

func (sr *ShowReminder) globalController(req *core.Request) (map[string]interface{}, *core.URL, error) {
	data := map[string]interface{}{
		"title":       "foo",
		"date":        time.Now(),
		"description": "ssssssss",
	}
	return data, nil, nil
}
