package pages

import (
	"reminder/core"
	"reminder/core/page"

	"github.com/pkg/errors"
)

type ReminderList struct {
	*page.BasePage
	PreviewTemplate string
}

func (rl *ReminderList) Init(builder *page.PagesBuilder) error {
	controllers := map[string]page.Controller{
		"on_get_or_delete": rl.getOrDeleteInputHandler,
	}
	var err error
	rl.BasePage, err = builder.NewBasePage("reminder_list", rl.globalController, controllers)
	if err != nil {
		return err
	}
	previewTemplate, _ := rl.ParsedPage.Config["preview_template"].(string)
	if len(previewTemplate) == 0 {
		return errors.Errorf("config doesn't contain preview template %v", rl.ParsedPage.Config)
	}
	rl.PreviewTemplate = previewTemplate
	return nil
}

func (rl *ReminderList) getOrDeleteInputHandler(req *core.Request) (map[string]interface{}, *core.URL, error) {
	return map[string]interface{}{"deleted": false, "reminder_id": 2}, nil, nil
}

func (rl *ReminderList) globalController(req *core.Request) (map[string]interface{}, *core.URL, error) {
	data := map[string]interface{}{
		"no_reminders":      false,
		"reminder_previews": []interface{}{"foo", "bbbbbb", "222"},
	}
	return data, nil, nil
}
