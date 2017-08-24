package pages

import (
	"reminder/core/page"
)

func NewNotFound(builder *page.PagesBuilder) (page.Page, error) {
	return builder.NewBasePage("not_found", nil, nil)
}
