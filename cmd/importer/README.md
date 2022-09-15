# importer

Is the storefront-api data importer for categories, brands, products, and stores.

### Usage

This service can be run by simply running `go run cmd/importer/main.go`, with the following flags:

NOTE: All flags are mandatory.

```
$ go run cmd/importer/main.go 
    -operation  Name of the import operation to perform.
                [ category | brand | product | store ]
    -path       Path to the file to be imported.
    -mongo      MongoDB connection URI string.
    -name       Database name where all the imported data will be stored to.
```