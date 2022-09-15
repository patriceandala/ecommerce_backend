package main

import (
	"context"
	"errors"
	"testing"

	"github.com/dropezy/storefront-backend/internal/storage/model/brand"
	"github.com/dropezy/storefront-backend/internal/storage/model/category"
	"github.com/dropezy/storefront-backend/internal/storage/model/product"
	"go.mongodb.org/mongo-driver/bson/primitive"

	// protobuf
	ctpb "github.com/dropezy/proto/v1/category"
)

func TestImportProducts(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		defaultTestTimeout,
	)
	t.Cleanup(cancel)

	// insert dummy category
	childCategory := category.Category{
		ID:         primitive.NewObjectID(),
		Level:      ctpb.CategoryLevel_CATEGORY_LEVEL_2,
		Name_ID:    "ID-product-test-child-category-name",
		Name_EN:    "EN-product-test-child-category-name",
		ImagesURLs: []string{"valid-image-url"},
	}
	parentCategory := &category.Category{
		ID:           primitive.NewObjectID(),
		Level:        ctpb.CategoryLevel_CATEGORY_LEVEL_1,
		Name_ID:      "ID-product-test-category-name",
		Name_EN:      "EN-product-test-category-name",
		Abbreviation: "DNE",
		ChildCategories: []category.Category{
			childCategory,
		},
	}
	if _, err := testDb.Collection(categoryCollection).
		InsertOne(ctx, parentCategory); err != nil {
		t.Fatalf("unexpected error, got = %v", err)
	}

	// insert dummy brand
	b := &brand.Brand{
		Name: "Dropezy",
	}
	if _, err := testDb.Collection(brandCollection).InsertOne(ctx, b); err != nil {
		t.Fatalf("unexpected error, got = %v", err)
	}

	// insert dummy variant type
	vt := &product.VariantType{
		Name: "UOM",
	}
	if _, err := testDb.Collection(variantTypeCollection).InsertOne(ctx, vt); err != nil {
		t.Fatalf("unexpected error, got = %v", err)
	}

	t.Run("Success", func(t *testing.T) {
		path := "testdata/product/success.csv"
		if err := importProducts(ctx, testDb, path); err != nil {
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
			name:    "EmptyProductNameEN",
			path:    "testdata/product/empty_product_name_en.csv",
			wantErr: errProductNameENIsRequired,
		},
		{
			name:    "EmptyProductNameID",
			path:    "testdata/product/empty_product_name_id.csv",
			wantErr: errProductNameIDIsRequired,
		},
		{
			name:    "EmptyShoptreeVariantID",
			path:    "testdata/product/empty_shoptree_variant_id.csv",
			wantErr: errShoptreeVariantIDIsRequired,
		},
		{
			name:    "EmptyVariantValue",
			path:    "testdata/product/empty_variant_value.csv",
			wantErr: errVariantValueIsRequired,
		},
		{
			name:    "EmptyVariantQuantifierEN",
			path:    "testdata/product/empty_variant_quantifier_en.csv",
			wantErr: errVariantQuantifierENIsRequired,
		},
		{
			name:    "EmptyVariantQuantifierID",
			path:    "testdata/product/empty_variant_quantifier_id.csv",
			wantErr: errVariantQuantifierIDIsRequired,
		},
		{
			name:    "EmptySKU",
			path:    "testdata/product/empty_sku.csv",
			wantErr: errSKUIsRequired,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := importProducts(ctx, testDb, test.path)
			if !errors.Is(errors.Unwrap(err), test.wantErr) {
				t.Fatalf("importProducts(_, _) error, got = %v, want = %v", err, test.wantErr)
			}
		})
	}
}
