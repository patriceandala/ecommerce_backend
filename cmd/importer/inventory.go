package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/dropezy/storefront-backend/internal/storage/model"
	"github.com/dropezy/storefront-backend/internal/storage/model/inventory"
	"github.com/dropezy/storefront-backend/internal/storage/model/product"

	// protobuf
	mpb "github.com/dropezy/proto/meta"
	prpb "github.com/dropezy/proto/v1/product"
)

const inventoryCollection = "inventory"

var (
	errInventoryIDIsRequired                 = errors.New("inventory id is required")
	errInventoryStoreIDIsRequired            = errors.New("inventory store id is required")
	errInventoryShoptreeLocationIDIsRequired = errors.New("inventory shoptree location id is required")
	errInventoryProductIsRequired            = errors.New("inventory product is required")

	errInventoryProductIDIsRequired                = errors.New("inventory product id is required")
	errInventoryProductPriceIsRequired             = errors.New("inventory product price is required")
	errInventoryProductProductIDIsRequired         = errors.New("inventory product product id is required")
	errInventoryProductVariantIDIsRequired         = errors.New("inventory product variant id is required")
	errInventoryProductShoptreeVariantIDIsRequired = errors.New("inventory product shoptree variant id is required")
)

// importInventories opens products file to get price and bulk inserts inventory.
// NOTE: Currently this is just a dummy importer
func importInventories(ctx context.Context, db *mongo.Database, path string) error {
	// check if stores exist in our database.
	darkstores, err := getStores(ctx, db)
	if err != nil {
		return err
	}
	_ = darkstores

	// check if products exist in our database.
	products, err := getProducts(ctx, db)
	if err != nil {
		return err
	}
	_ = products

	// open products file to get price (currently price is stored in products file)
	// TODO(wilson): change this to inventories file
	productsFile, err := os.Open(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("failed to open products file: %w", err)
	}
	defer func() {
		err := productsFile.Close()
		if err != nil {
			log.Fatalf("failed to close categories file: %v", err)
		}
	}()

	// reads the products file
	// TODO(wilson): change this to inventories file
	productsFileLines, err := csv.NewReader(productsFile).ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read products file lines: %w", err)
	}

	var (
		product_name_EN          int
		product_variant_id       int
		variant_product_sellable int
		price                    int
	)

	// TODO(wilson): update this to list of inventories.
	inv := &inventory.Inventory{
		ID:                 primitive.NewObjectID(),
		StoreID:            darkstores[0].ID,
		ShoptreeLocationID: darkstores[0].ShoptreeLocationID,
		Products:           []*inventory.Product{},
	}
	for i, line := range productsFileLines {
		rowNumber := i + 1
		if rowNumber == 1 {
			// get required index for headers
			for idx, header := range line {
				switch header {
				case "product_name_ENG":
					product_name_EN = idx
				case "product_variant_id":
					product_variant_id = idx
				case "variant_product_sellable":
					variant_product_sellable = idx
				case "selling_price":
					price = idx
				}
			}
			continue
		}

		// look for product in products
		p, err := getProduct(products, line[product_name_EN], rowNumber)
		if err != nil {
			return err
		}

		// look for product variant in product
		pv, err := getProductVariant(p.Variants, line[product_variant_id], rowNumber)
		if err != nil {
			fmt.Printf("%+v PV\n", p.Variants[0].ShoptreeVariantID)
			fmt.Println(line[product_variant_id])
			return err
		}

		inventoryProduct := &inventory.Product{
			ID:    primitive.NewObjectID(),
			Stock: int32(10), // TODO(wilson): update this to use real stock data
			Price: &model.Amount{
				Num: line[price] + "00",
				Cur: mpb.Currency_CURRENCY_IDR,
			},
			ProductID:         p.ID,
			VariantID:         pv.ID,
			ShoptreeVariantID: pv.ShoptreeVariantID,
		}

		if strings.Compare(line[variant_product_sellable], "yes") == 0 {
			inventoryProduct.Status = prpb.ProductStatus_PRODUCT_STATUS_ENABLED
		} else {
			inventoryProduct.Status = prpb.ProductStatus_PRODUCT_STATUS_DISABLED
		}

		if err := validateInventoryProduct(inventoryProduct); err != nil {
			return err
		}

		inv.Products = append(inv.Products, inventoryProduct)
	}

	if err := validateInventory(inv); err != nil {
		return err
	}

	collection := db.Collection(inventoryCollection)

	// TODO(wilson): change this to bulk write
	// insert inventory
	if _, err := collection.InsertOne(ctx, inv); err != nil {
		return fmt.Errorf("failed to execute InsertOne, on import inventories: %w", err)
	}

	log.Print("successfully imported inventories!")
	return nil
}

// getProducts get list of products saved in our database.
func getProducts(ctx context.Context, db *mongo.Database) ([]*product.Product, error) {
	cur, err := db.Collection(productCollection).Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to execute ")
	}

	var products []*product.Product
	if err := cur.All(ctx, &products); err != nil {
		return nil, fmt.Errorf("failed to decode products: %w", err)
	}
	if len(products) < 1 {
		return nil, errors.New("no products found in our database")
	}

	return products, nil
}

// getProduct looks for product with "Name_EN" on products list in our db.
func getProduct(products []*product.Product, name string, rowNumber int) (*product.Product, error) {
	for _, p := range products {
		if strings.Compare(p.Name_EN, name) == 0 {
			return p, nil
		}
	}
	return nil, fmt.Errorf("failed to find product with EN name: %s, on row: %d", name, rowNumber)
}

// getProductVariant looks for product variant with "ShoptreeVariantID" in a product.
func getProductVariant(variants []*product.ProductVariant, shoptreeVariantID string, rowNumber int) (*product.ProductVariant, error) {
	for _, pv := range variants {
		if strings.Compare(pv.ShoptreeVariantID, shoptreeVariantID) == 0 {
			return pv, nil
		}
	}
	return nil, fmt.Errorf("failed to find product variant with shoptree variant id: %s, on row: %d", shoptreeVariantID, rowNumber)
}

// validateInventory checks whether all inventory parameters are valid.
func validateInventory(i *inventory.Inventory) error {
	switch {
	case i.ID == primitive.NilObjectID:
		return errInventoryIDIsRequired
	case i.StoreID == primitive.NilObjectID:
		return errInventoryStoreIDIsRequired
	case i.ShoptreeLocationID == "":
		return errInventoryShoptreeLocationIDIsRequired
	case len(i.Products) < 1:
		return errInventoryProductIsRequired
	}
	return nil
}

// validateInventoryProduct checks whether all inventory product parameters are valid.
func validateInventoryProduct(p *inventory.Product) error {
	switch {
	case p.ID == primitive.NilObjectID:
		return errInventoryProductIDIsRequired
	case p.Price == nil:
		return errInventoryProductPriceIsRequired
	case p.ProductID == primitive.NilObjectID:
		return errInventoryProductProductIDIsRequired
	case p.VariantID == primitive.NilObjectID:
		return errInventoryProductVariantIDIsRequired
	case p.ShoptreeVariantID == "":
		return errInventoryProductShoptreeVariantIDIsRequired
	}
	return nil
}
