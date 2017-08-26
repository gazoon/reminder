package pages

import (
	"reminder/core/page"
)

type NotFound struct {
	*page.BasePage
}

func (nf *NotFound) Init(builder *page.PagesBuilder) error {
	var err error
	nf.BasePage, err = builder.NewBasePage("not_found", nil, nil)
	return err
}
