// Package services is a utility package to register
// storefront services to the gRPC server.
package services

import "google.golang.org/grpc"

type Svc func(srv *grpc.Server) error

// Register creates and registers dropezy storefront backend grpc services.
func Register(srv *grpc.Server, services ...Svc) error {
	for _, svc := range services {
		if err := svc(srv); err != nil {
			return err
		}
	}
	return nil
}
