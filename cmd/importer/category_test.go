package main

import (
	"context"
	"errors"
	"testing"
)

func TestImportCategories(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		defaultTestTimeout,
	)
	t.Cleanup(cancel)

	t.Run("Success", func(t *testing.T) {
		t.Parallel()

		path := "testdata/category/success.csv"
		if err := importCategories(ctx, testDb, path); err != nil {
			t.Fatalf("expected nil error, got = %v", err)
		}
	})

	// failed scenarios
	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		{
			name:    "EmptyCategoryNameEN",
			path:    "testdata/category/empty_category_name_en.csv",
			wantErr: errCategoryNameENIsRequired,
		},
		{
			name:    "EmptyCategoryNameID",
			path:    "testdata/category/empty_category_name_id.csv",
			wantErr: errCategoryNameIDIsRequired,
		},
		{
			name:    "EmptyAbbreviation",
			path:    "testdata/category/empty_abbreviation.csv",
			wantErr: errAbbreviationIsRequired,
		},
		{
			name:    "EmptyCategoryImageURL",
			path:    "testdata/category/empty_category_image_url.csv",
			wantErr: errCategoryImageURLIsRequired,
		},
		{
			name:    "EmptySubcategoryNameEN",
			path:    "testdata/category/empty_subcategory_name_en.csv",
			wantErr: errSubcategoryNameENIsRequired,
		},
		{
			name:    "EmptySubcategoryNameID",
			path:    "testdata/category/empty_subcategory_name_id.csv",
			wantErr: errSubcategoryNameIDIsRequired,
		},
		{
			name:    "EmptySubcategoryImageURL",
			path:    "testdata/category/empty_subcategory_image_url.csv",
			wantErr: errSubcategoryImageURLIsRequired,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := importCategories(ctx, testDb, test.path)
			if !errors.Is(errors.Unwrap(err), test.wantErr) {
				t.Fatalf("importCategories(_, _) error, got = %v, want = %v", err, test.wantErr)
			}
		})
	}
}
