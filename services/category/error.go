package category

import "errors"

var (
	ErrStoreIDIsRequired  = errors.New("store id is required")
	ErrCategoryIDNotFound = errors.New("category id not found")
)
