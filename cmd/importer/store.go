package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/dropezy/storefront-backend/internal/storage/model/darkstore"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const darkstoreCollection = "store"

var (
	errStoreNotFound = errors.New("no stores found in our database")
)

func importStore(ctx context.Context, db *mongo.Database, path string) error {
	// check if stores exist in our database
	darkstores, err := getStores(ctx, db)
	if err != nil && !errors.Is(err, errStoreNotFound) {
		return err
	}
	if len(darkstores) > 0 {
		return errors.New("darkstore already exist in our database")
	}

	collection := db.Collection(darkstoreCollection)

	// if no darkstores found, insert darkstore
	// NOTE: This is just a dummy darkstore, need to be insert from path
	ds := &darkstore.Store{
		ID:                 primitive.NewObjectID(),
		Name:               "Dropezy Store",
		ShoptreeLocationID: "962553ec420c45388a6bfb26308bdc23",
		LocationCode:       "WHT",
	}
	if _, err := collection.InsertOne(ctx, ds); err != nil {
		return fmt.Errorf("failed to execute InsertOne store: %w", err)
	}

	log.Print("successfully imported stores!")
	return nil
}

// getStores get list of darkstores saved in our database.
func getStores(ctx context.Context, db *mongo.Database) ([]*darkstore.Store, error) {
	cur, err := db.Collection(darkstoreCollection).Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to execute Find stores: %w", err)
	}

	var darkstores []*darkstore.Store
	if err := cur.All(ctx, &darkstores); err != nil {
		return nil, fmt.Errorf("failed to decode darkstores: %w", err)
	}
	if len(darkstores) < 1 {
		return nil, errStoreNotFound
	}

	return darkstores, nil
}
