package pages

import (
	"reminder/core"
	"reminder/core/page"
	"reminder/storages/chats"
)

type ChangeTimezone struct {
	*page.BasePage

	Chats chats.Storage
}

func (ct *ChangeTimezone) Init(builder *page.PagesBuilder) error {
	controllers := map[string]page.Controller{
		"on_timezone": ct.onTimezoneController,
	}
	var err error
	ct.BasePage, err = builder.NewBasePage("change_timezone", nil, controllers)
	return err
}

func (ct *ChangeTimezone) onTimezoneController(req *core.Request) (map[string]interface{}, *core.URL, error) {
	ct.GetLogger(req.Ctx).Infof("on timezone input: %s", req.Msg.Text)
	return nil, nil, nil
}
