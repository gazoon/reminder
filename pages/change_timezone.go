package pages

import (
	"reminder/core"
	"reminder/core/page"
)

type ChangeTimezonePage struct {
	*page.BasePage
}

func NewChangeTimezone(builder *page.PagesBuilder) (page.Page, error) {
	p := new(ChangeTimezonePage)
	controllers := map[string]page.Controller{
		"on_timezone": p.onTimezoneController,
	}
	var err error
	p.BasePage, err = builder.NewBasePage("change_timezone", nil, controllers)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (ct *ChangeTimezonePage) onTimezoneController(req *core.Request) (map[string]interface{}, *core.URL, error) {
	ct.GetLogger(req.Ctx).Infof("on timezone input: %s", req.Msg.Text)
	return nil, nil, nil
}
