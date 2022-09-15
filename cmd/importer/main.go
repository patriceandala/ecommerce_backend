package main

import (
	"context"
	"flag"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

const (
	operationCategory  = "category"
	operationBrand     = "brand"
	operationProduct   = "product"
	operationStore     = "store"
	operationInventory = "inventory"
)

var (
	defaultTimeout = 60 * time.Second
)

func main() {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		defaultTimeout,
	)
	defer cancel()

	operationFlag := flag.String("operation", "", "bulk insert operation type")
	pathFlag := flag.String("path", "", "absolute path for data")
	databaseNameFlag := flag.String("name", "", "database name")

	// format: mongodb://[username:password@]host1[:port1][,...hostN[:portN]][/[defaultauthdb][?options]]
	// e.g. mongodb://username:password@127.0.0.1:27017?retryWrites=true
	mongoFlag := flag.String("mongo", "", "mongodb uri")

	flag.Parse()

	// path and mongo flag is required
	switch "" {
	case *pathFlag:
		log.Fatal("path flag cannot be empty")
	case *mongoFlag:
		log.Fatal("mongo flag cannot be empty")
	case *databaseNameFlag:
		log.Fatal("database name flag cannot be empty")
	}

	db := newMongoDb(*mongoFlag, *databaseNameFlag)

	switch *operationFlag {
	case operationCategory:
		if err := importCategories(ctx, db, *pathFlag); err != nil {
			log.Fatalf("failed to import categories: %v", err)
		}
	case operationProduct:
		if err := importProducts(ctx, db, *pathFlag); err != nil {
			log.Fatalf("failed to import products: %v", err)
		}
	case operationBrand:
		// TODO(wilson): unimplemented
		log.Fatal("currently import brands is unimplemented")
	case operationStore:
		// TODO(wilson): update to real store importer implementation
		if err := importStore(ctx, db, *pathFlag); err != nil {
			log.Fatalf("failed to import stores: %v", err)
		}
	case operationInventory:
		// TODO(wilson): update to real inventory importer implementation
		if err := importInventories(ctx, db, *pathFlag); err != nil {
			log.Fatalf("failed to import inventories: %v", err)
		}
	default:
		log.Fatal("invalid operation flag\nvalid flags: category, brand, product, store")
	}
}

func newMongoDb(uri, dbName string) *mongo.Database {
	clientOpts := options.Client().ApplyURI(uri)

	const connectTimeout = 10 * time.Second
	ctx, cancel := context.WithTimeout(
		context.Background(),
		connectTimeout,
	)
	defer cancel()

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		log.Fatalf("failed to connect to mongo instance: %v", err)
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		log.Fatalf("failed to verify mongo client connection: %v", err)
	}

	db := client.Database(dbName)

	return db
}
