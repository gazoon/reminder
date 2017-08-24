package pages

import (
	"reminder/core/page"
)

func NewHomePage(builder *page.PagesBuilder) (page.Page, error) {
	return builder.NewBasePage("home", nil, nil)
}
