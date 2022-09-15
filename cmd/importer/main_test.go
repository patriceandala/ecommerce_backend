package main

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var (
	testDb             *mongo.Database
	defaultTestTimeout = 10 * time.Second
)

func TestMain(m *testing.M) {
	const dbConnEnv = "MONGO_CONNECTION"
	dbstring := os.Getenv(dbConnEnv)
	if dbstring == "" {
		log.Printf("%s is not set, skipping", dbConnEnv)
		return
	}

	clientOpts := options.Client().
		ApplyURI(dbstring).
		// set test instance to read most recent data in the replica
		// to make sure our tests results are consistent, since we don't want to deal with flakines on read/write operations on this level
		SetReadConcern(readconcern.Local())

	const connectTimeout = 10 * time.Second
	ctx, cancel := context.WithTimeout(
		context.Background(), connectTimeout,
	)
	defer cancel()

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		panic(err)
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		panic(err)
	}

	dbName := uuid.New().String()
	testDb = client.Database(dbName)

	teardown := func() {
		if err := testDb.Drop(context.Background()); err != nil {
			panic(err)
		}
	}

	exitCode := m.Run()

	teardown()

	os.Exit(exitCode)
}
