package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/dropezy/storefront-backend/internal/storage/model/brand"
	"github.com/dropezy/storefront-backend/internal/storage/model/category"
	"github.com/dropezy/storefront-backend/internal/storage/model/product"

	// protobuf
	prpb "github.com/dropezy/proto/v1/product"
)

const (
	productCollection     = "product"
	brandCollection       = "brand"
	variantTypeCollection = "variant_type"
)

var (
	errProductIDIsRequired          = errors.New("product id is required")
	errProductNameENIsRequired      = errors.New("product name EN is required")
	errProductNameIDIsRequired      = errors.New("product name ID is required")
	errProductBrandIDIsRequired     = errors.New("product brand id is required")
	errProductCategory1IDIsRequired = errors.New("product category 1 id is required")
	errProductCategory2IDIsRequired = errors.New("product categories 2 id is required")
	errProductVariantsIsRequired    = errors.New("product variants is required")

	errProductVariantIDIsRequired            = errors.New("product variant id is required")
	errShoptreeVariantIDIsRequired           = errors.New("shoptree variant id is required")
	errProductVariantImageURLIsRequired      = errors.New("product variant image url is required")
	errProductVariantVariantTypeIDIsRequired = errors.New("product variant variant id is required")
	errVariantValueIsRequired                = errors.New("variant value is required")
	errVariantQuantifierIDIsRequired         = errors.New("variant quantifier ID is required")
	errVariantQuantifierENIsRequired         = errors.New("variant quantifier EN is required")
	errSKUIsRequired                         = errors.New("sku is required")
	errSKUAndCategoryAbbreviationMismatch    = errors.New("sku and category abbreviation mismatch")
	errBarcodeIsRequired                     = errors.New("barcode is required")
)

type HeadersIndex struct {
	shoptree_variant_id   int
	sku                   int
	product_name_EN       int
	product_name_ID       int
	variant_value         int
	variant_quantifier_EN int
	variant_quantifier_ID int
	maximum_order         int
	barcode               int
	category_name_EN      int
	subcategory_name_EN   int
	description_EN        int
	description_ID        int
	image_url             int
	default_variant       int
}

// importProducts looks into the path given and bulk insert all the products.
func importProducts(ctx context.Context, db *mongo.Database, path string) error {
	// first get all categories from database.
	categories, err := getCategories(ctx, db)
	if err != nil {
		return err
	}

	// find or insert brand
	// NOTE: This is only temporary, until brand lists are ready to be imported.
	brandData, err := findOrInsertBrand(ctx, db)
	if err != nil {
		return err
	}

	// find or insert variant type
	variantTypeData, err := findOrInsertVariantType(ctx, db)
	if err != nil {
		return err
	}

	// open products file from the path given.
	productsFile, err := os.Open(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("failed to open products file: %w", err)
	}
	defer func() {
		err := productsFile.Close()
		if err != nil {
			log.Fatalf("failed to close products file: %v", err)
		}
	}()

	// reads the products file.
	productsFileData, err := csv.NewReader(productsFile).ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read products file lines: %w", err)
	}

	logList := []string{}

	var products []*product.Product
	hi := &HeadersIndex{}
	for i, line := range productsFileData {
		rowNumber := i + 1
		if rowNumber == 1 {
			// get all the index for the headers
			hi = getProductsHeaderIndex(line)
			continue
		}

		// check barcode and image url, product without barcode/image_url will be skipped
		// NOTE: This is just an extra measure to prevent invalid product insertion.
		switch "" {
		case line[hi.image_url]:
			logList = append(logList, fmt.Errorf("image url on row: %d, is empty, skipping product", rowNumber).Error())
			continue
		case line[hi.barcode]:
			logList = append(logList, fmt.Errorf("barcode on row: %d, is empty, skipping product", rowNumber).Error())
			continue
		}

		// convert max number to type int32
		maxNumber, err := strconv.ParseInt(line[hi.maximum_order], 10, 32)
		if err != nil {
			// return fmt.Errorf("failed to convert maximum order on row: %v, err: %w", i+1, err)
			logList = append(logList, fmt.Errorf("failed to convert maximum order on row: %d, err: %w", rowNumber, err).Error())
		}

		// look for product in products
		var productIdx int
		found := false
		for i, p := range products {
			if strings.Compare(p.Name_EN, line[hi.product_name_EN]) == 0 {
				productIdx = i
				found = true
				break
			}
		}

		variantImageURL := line[hi.sku] + "-0.webp"
		productVariant := &product.ProductVariant{
			ID:                   primitive.NewObjectID(),
			ShoptreeVariantID:    line[hi.shoptree_variant_id],
			ImagesURLs:           []string{variantImageURL},
			VariantTypeID:        variantTypeData.ID,
			VariantValue:         line[hi.variant_value],
			VariantQuantifier_ID: line[hi.variant_quantifier_ID],
			VariantQuantifier_EN: line[hi.variant_quantifier_EN],
			MaximumOrder:         int32(maxNumber), // maximum order 0 means there is no limit
			SKU:                  line[hi.sku],
			Barcode:              line[hi.barcode],
		}
		// check if default variant
		if strings.Compare(line[hi.default_variant], "yes") == 0 {
			productVariant.VariantStatus = prpb.VariantStatus_VARIANT_STATUS_DEFAULT
		}
		if err := validateProductVariant(productVariant); err != nil {
			return fmt.Errorf("failed to import products, on row: %d, err: %w", rowNumber, err)
		}

		// if product has been created, append product variant to found product,
		// else, create product and insert product variant to variants list.
		if found {
			products[productIdx].Variants =
				append(products[productIdx].Variants, productVariant)
		} else {
			// look for level 1 category
			categoryl1, err := getParentCategory(categories, line[hi.category_name_EN], rowNumber)
			if err != nil {
				// this is just a measure to make sure the program continues and keeps
				// logging the errors found.
				categoryl1 = &category.Category{
					ID: primitive.NewObjectID(),
				}
				logList = append(logList, err.Error())
			}

			// look for C2 category
			categoryl2, err := getChildCategory(categoryl1.ChildCategories, line[hi.subcategory_name_EN], rowNumber)
			if err != nil {
				categoryl2 = category.Category{
					ID: primitive.NewObjectID(),
				}
				// skip logging if child categories is empty.
				if len(categoryl1.ChildCategories) > 0 {
					logList = append(logList, err.Error())
				}
			}

			// check if variant SKU is designated with the correct category.
			if !strings.Contains(line[hi.sku], categoryl1.Abbreviation) {
				return fmt.Errorf("mismatch sku: %s and category abbreviation: %s", line[hi.sku], categoryl1.Abbreviation)
			}

			p := &product.Product{
				ID:          primitive.NewObjectID(),
				Name_ID:     line[hi.product_name_ID],
				Name_EN:     line[hi.product_name_EN],
				BrandID:     brandData.ID,
				Category1ID: categoryl1.ID,
				Category2ID: categoryl2.ID,
				Variants:    []*product.ProductVariant{productVariant},
			}
			// if description is #N/A, means description is empty
			emptyDescValue := "#N/A"
			if strings.Compare(line[hi.description_EN], emptyDescValue) != 0 {
				p.Description_EN = line[hi.description_EN]
			}
			if strings.Compare(line[hi.description_ID], emptyDescValue) != 0 {
				p.Description_ID = line[hi.description_ID]
			}
			if err := validateProduct(p); err != nil {
				return fmt.Errorf("failed to import products, on row: %d, err: %w", rowNumber, err)
			}
			products = append(products, p)
		}
	}

	// if log list is not empty, fails bulk insert and return errors.
	if len(logList) > 0 {
		log.Print(strings.Join(logList, "\n"))
		return errors.New("found several errors while importing products")
	}

	collection := db.Collection(productCollection)

	// converts all products to mongo write model for bulk insert.
	models := []mongo.WriteModel{}
	for _, p := range products {
		writeModel := mongo.NewInsertOneModel().SetDocument(p)
		models = append(models, writeModel)
	}

	// bulk insert products
	if _, err := collection.BulkWrite(ctx, models); err != nil {
		return fmt.Errorf("failed to execute BulkWrite, on import products: %w", err)
	}

	log.Print("successfully imported products!")
	return nil
}

// getCategories fetches all categories in our database which will be used to get
// level 1 and 2 categories for product bulk insert.
func getCategories(ctx context.Context, db *mongo.Database) ([]*category.Category, error) {
	cursor, err := db.Collection(categoryCollection).Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to execute Find categories: %w", err)
	}

	var categories []*category.Category
	for cursor.Next(ctx) {
		c := &category.Category{}
		if err := cursor.Decode(c); err != nil {
			return nil, fmt.Errorf("failed to decode category: %w", err)
		}
		categories = append(categories, c)
	}

	// categories must exist before in order to execute bulk insert products.
	if len(categories) < 1 {
		return nil, fmt.Errorf("no categories found in database")
	}

	return categories, nil
}

// findOrInsertBrand first checks whether the brand already exist in our database,
// if it doesn't exist then insert the brand and return brand response.
func findOrInsertBrand(ctx context.Context, db *mongo.Database) (*brand.Brand, error) {
	// check whether brand already exist
	brandData := &brand.Brand{}
	found := true
	if err := db.Collection(brandCollection).
		FindOne(ctx, bson.M{
			"name": "Dropezy",
		}).
		Decode(brandData); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			found = false
		} else {
			return nil, fmt.Errorf("failed to execute FindOne brand: %w", err)
		}
	}
	// if brand doesn't exist, seed one dummy brand data for all products.
	if !found {
		brandRes, err := db.Collection(brandCollection).InsertOne(ctx, &brand.Brand{
			Name: "Dropezy",
		})
		if err != nil {
			return nil, fmt.Errorf("failed to execute InsertOne brand: %w", err)
		}
		brandData = &brand.Brand{
			ID: brandRes.InsertedID.(primitive.ObjectID),
		}
	}
	return brandData, nil
}

// findOrInsertVariantType first checks whether the variant type already exist in our database,
// if it doesn't exist then insert the variant type and return variant type response.
func findOrInsertVariantType(ctx context.Context, db *mongo.Database) (*product.VariantType, error) {
	// check whether variant type exist
	variantType := &product.VariantType{}
	found := true
	if err := db.Collection(variantTypeCollection).
		FindOne(ctx, bson.M{
			"name": "UOM",
		}).
		Decode(variantType); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			found = false
		} else {
			return nil, fmt.Errorf("failed to execute FindOne variant type: %w", err)
		}
	}
	// if variant type doesn't exist, insert variant type
	if !found {
		variantTypeRes, err := db.Collection(variantTypeCollection).InsertOne(ctx, &product.VariantType{
			Name: "UOM",
		})
		if err != nil {
			return nil, fmt.Errorf("failed to execute InsertOne variant type: %w", err)
		}
		variantType = &product.VariantType{
			ID: variantTypeRes.InsertedID.(primitive.ObjectID),
		}
	}
	return variantType, nil
}

// getParentCategory looks for a category with "Name_EN" on categories list in our db.
func getParentCategory(categories []*category.Category, ctName string, rowNumber int) (*category.Category, error) {
	for _, c := range categories {
		if strings.Compare(c.Name_EN, ctName) == 0 {
			return c, nil
		}
	}
	return nil, fmt.Errorf("failed to find level 1 category with EN name: %s, on row: %d", ctName, rowNumber)
}

// getChildCategory looks for a child category with "Name_EN" on categories list in our db.
func getChildCategory(categories []category.Category, ctName string, rowNumber int) (category.Category, error) {
	for _, c := range categories {
		if strings.Compare(c.Name_EN, ctName) == 0 {
			return c, nil
		}
	}
	return category.Category{}, fmt.Errorf("failed to find level 2 category with EN name: %s, on row: %d", ctName, rowNumber)
}

// validateProduct checks whether all required parameters are fulfilled.
func validateProduct(p *product.Product) error {
	switch {
	case p.ID == primitive.NilObjectID: // just in case id is not properly generated.
		return errProductIDIsRequired
	case p.Name_ID == "":
		return errProductNameIDIsRequired
	case p.Name_EN == "":
		return errProductNameENIsRequired
	case p.BrandID == primitive.NilObjectID:
		return errProductBrandIDIsRequired
	case p.Category1ID == primitive.NilObjectID:
		return errProductCategory1IDIsRequired
	case p.Category2ID == primitive.NilObjectID:
		return errProductCategory2IDIsRequired
	case p.Variants == nil:
		return errProductVariantsIsRequired
	}
	return nil
}

// validateProductVariant checks whether all required parameters are fulfilled.
func validateProductVariant(pv *product.ProductVariant) error {
	switch {
	case pv.ID == primitive.NilObjectID:
		return errProductVariantIDIsRequired
	case pv.ShoptreeVariantID == "":
		return errShoptreeVariantIDIsRequired
	case pv.ImagesURLs[0] == "":
		return errProductVariantImageURLIsRequired
	case len(pv.ImagesURLs) < 1: // just in case image url is not inserted.
		return errProductVariantImageURLIsRequired
	case pv.VariantTypeID == primitive.NilObjectID:
		return errProductVariantVariantTypeIDIsRequired
	case pv.VariantValue == "":
		return errVariantValueIsRequired
	case pv.VariantQuantifier_ID == "":
		return errVariantQuantifierIDIsRequired
	case pv.VariantQuantifier_EN == "":
		return errVariantQuantifierENIsRequired
	case pv.SKU == "":
		return errSKUIsRequired
	case pv.Barcode == "":
		return errBarcodeIsRequired
	}

	return nil
}

// get the index of products file headers.
func getProductsHeaderIndex(line []string) *HeadersIndex {
	hi := &HeadersIndex{}
	for idx, header := range line {
		switch header {
		case "product_variant_id":
			hi.shoptree_variant_id = idx
		case "sku_structured":
			hi.sku = idx
		case "product_name_ENG":
			hi.product_name_EN = idx
		case "product_name_IND":
			hi.product_name_ID = idx
		case "option_value_1":
			hi.variant_value = idx
		case "quantifier_ENG":
			hi.variant_quantifier_EN = idx
		case "quantifier_IND":
			hi.variant_quantifier_ID = idx
		case "maximum_ordered_qty":
			hi.maximum_order = idx
		case "barcodes":
			hi.barcode = idx
		case "category_name_EN":
			hi.category_name_EN = idx
		case "sub_category_name_EN":
			hi.subcategory_name_EN = idx
		case "product_description_IND":
			hi.description_ID = idx
		case "product_description_ENG":
			hi.description_EN = idx
		case "image_link":
			hi.image_url = idx
		case "default_variant":
			hi.default_variant = idx
		}
	}
	return hi
}
