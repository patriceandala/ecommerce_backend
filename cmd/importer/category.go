package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/dropezy/storefront-backend/internal/storage/model/category"

	// protobuf
	ctpb "github.com/dropezy/proto/v1/category"
)

const categoryCollection = "category"

var (
	errCategoryNameENIsRequired   = errors.New("category name EN is required")
	errCategoryNameIDIsRequired   = errors.New("category name ID is required")
	errAbbreviationIsRequired     = errors.New("abbreviation is required")
	errCategoryImageURLIsRequired = errors.New("category image url is required")

	errSubcategoryNameENIsRequired   = errors.New("subcategory name EN is required")
	errSubcategoryNameIDIsRequired   = errors.New("subcategory name ID is required")
	errSubcategoryImageURLIsRequired = errors.New("subcategory image url is required")
)

// importCategories looks into the path given and bulk insert all the categories.
func importCategories(ctx context.Context, db *mongo.Database, path string) error {
	categoriesFile, err := os.Open(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("failed to open categories file: %w", err)
	}
	defer func() {
		err := categoriesFile.Close()
		if err != nil {
			log.Fatalf("failed to close categories file: %v", err)
		}
	}()

	// reads the category file
	categoriesFileLines, err := csv.NewReader(categoriesFile).ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read categories file lines: %w", err)
	}

	var (
		category_name_EN    int
		category_name_ID    int
		abbreviation        int
		subcategory_name_EN int
		subcategory_name_ID int
	)

	var categories []*category.Category
	for i, line := range categoriesFileLines {
		rowNumber := i + 1
		if rowNumber == 1 {
			// get all the index for the headers
			for idx, header := range line {
				switch header {
				case "category_name_EN":
					category_name_EN = idx
				case "category_name_ID":
					category_name_ID = idx
				case "abbreviation":
					abbreviation = idx
				case "subcategory_name_EN":
					subcategory_name_EN = idx
				case "subcategory_name_ID":
					subcategory_name_ID = idx
				}
			}
			continue
		}

		// look for c1 category in categories
		var categoriesIdx int
		found := false
		for i, c := range categories {
			if c.Abbreviation == line[abbreviation] {
				// extra validation to check category name, when abbreviation is the same
				if c.Name_EN != line[category_name_EN] {
					return fmt.Errorf("abbreviation shown is the same on two different categories, on row: %d", rowNumber)
				}

				categoriesIdx = i
				found = true
				break
			}
		}

		categoryl2ID := primitive.NewObjectID()
		categoryl2ImageURL := categoryl2ID.Hex() + "-0.webp"
		categoryl2 := category.Category{
			ID:         categoryl2ID,
			Level:      ctpb.CategoryLevel_CATEGORY_LEVEL_2,
			Name_EN:    line[subcategory_name_EN],
			Name_ID:    line[subcategory_name_ID],
			ImagesURLs: []string{categoryl2ImageURL},
		}
		// check categoryl2 parameter values
		if err := validateC2(&categoryl2); err != nil {
			return fmt.Errorf("error on row: %d, error: %w", rowNumber, err)
		}

		if found {
			// append c2 categories to c1.
			categories[categoriesIdx].ChildCategories =
				append(categories[categoriesIdx].ChildCategories, categoryl2)
		} else {
			categoryl1ID := primitive.NewObjectID()
			categoryl1ImageURL := categoryl1ID.Hex() + "-0.webp"
			categoryl1 := &category.Category{
				ID:              categoryl1ID,
				Level:           ctpb.CategoryLevel_CATEGORY_LEVEL_1,
				Name_EN:         line[category_name_EN],
				Name_ID:         line[category_name_ID],
				Abbreviation:    line[abbreviation],
				ImagesURLs:      []string{categoryl1ImageURL},
				ChildCategories: []category.Category{categoryl2},
			}
			// check categoryl1 parameter values
			if err := validateC1(categoryl1); err != nil {
				return fmt.Errorf("error on row: %d, error: %w", rowNumber, err)
			}
			categories = append(categories, categoryl1)
		}
	}

	collection := db.Collection(categoryCollection)

	// converts all categories to mongo write model for bulk insert.
	models := []mongo.WriteModel{}
	for _, c := range categories {
		writeModel := mongo.NewInsertOneModel().SetDocument(c)
		models = append(models, writeModel)
	}

	// bulk insert categories.
	if _, err = collection.BulkWrite(ctx, models); err != nil {
		return fmt.Errorf("failed to execute BulkWrite, on import categories: %w", err)
	}

	log.Print("successfully imported categories!")
	return nil
}

// validateC1 checks whether all required parameters are fulfilled
func validateC1(ct *category.Category) error {
	switch "" {
	case ct.Name_EN:
		return errCategoryNameENIsRequired
	case ct.Name_ID:
		return errCategoryNameIDIsRequired
	case ct.Abbreviation:
		return errAbbreviationIsRequired
	// for now checking the 0 index should suffice
	case ct.ImagesURLs[0]:
		return errCategoryImageURLIsRequired
	}
	return nil
}

// validateC2 checks whether all required parameters are fulfilled
func validateC2(ct *category.Category) error {
	switch "" {
	case ct.Name_EN:
		return errSubcategoryNameENIsRequired
	case ct.Name_ID:
		return errSubcategoryNameIDIsRequired
	// for now checking the 0 index should suffice
	case ct.ImagesURLs[0]:
		return errSubcategoryImageURLIsRequired
	}
	return nil
}
