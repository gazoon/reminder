package pages

import (
	"reminder/core"
	"reminder/core/page"

	"github.com/pkg/errors"
)

type ReminderListPage struct {
	*page.BasePage
	PreviewTemplate string
}

func NewReminderListPage(builder *page.PagesBuilder) (page.Page, error) {
	p := new(ReminderListPage)
	controllers := map[string]page.Controller{
		core.DefaultAction: p.mainAction,
		"has_reminders":    p.fooController,
		"no_reminders":     p.barController,
		"on_get_or_delete": p.getOrDeleteInputHandler,
	}
	var err error
	p.BasePage, err = builder.NewBasePage("reminder_list", p.globalController, controllers)
	if err != nil {
		return nil, err
	}
	previewTemplate, _ := p.ParsedPage.Config["preview_template"].(string)
	if len(previewTemplate) == 0 {
		return nil, errors.Errorf("config doesn't contain preview template %v", p.ParsedPage.Config)
	}
	p.PreviewTemplate = previewTemplate
	return p, nil
}

func (rl *ReminderListPage) getOrDeleteInputHandler(req *core.Request) (map[string]interface{}, *core.URL, error) {
	return map[string]interface{}{"deleted": false, "reminder_id": 2}, nil, nil
}

func (rl *ReminderListPage) mainAction(req *core.Request) (map[string]interface{}, *core.URL, error) {
	data := map[string]interface{}{
		"has_reminders": true,
		"foo":           map[string]interface{}{"bar": []interface{}{2, 3, "4"}},
	}
	return data, nil, nil
}

func (rl *ReminderListPage) fooController(req *core.Request) (map[string]interface{}, *core.URL, error) {
	data := map[string]interface{}{
		"foo": "foo",
	}
	return data, nil, nil
}

func (rl *ReminderListPage) barController(req *core.Request) (map[string]interface{}, *core.URL, error) {
	data := map[string]interface{}{
		"foo": "bar",
	}
	return data, nil, nil
}

func (rl *ReminderListPage) globalController(req *core.Request) (map[string]interface{}, *core.URL, error) {
	data := map[string]interface{}{
		"reminder_previews": []interface{}{"foo", "bbbbbb", "222"},
	}
	return data, nil, nil
}
