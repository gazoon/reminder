package pages

import (
	"reminder/core/page"
)

type Home struct {
	*page.BasePage
}

func (h *Home) Init(builder *page.PagesBuilder) error {
	var err error
	h.BasePage, err = builder.NewBasePage("home", nil, nil)
	return err
}
