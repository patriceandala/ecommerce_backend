// Package product implements product gRPC service methods
// to manage dropezy product and product variants operations.
package product

import (
	"context"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dropezy/storefront-backend/internal/storage"
	"github.com/dropezy/storefront-backend/internal/storage/model/product"

	// protobuf
	prpb "github.com/dropezy/proto/ems/v1/product"

	// old protobuf
	s_ctpb "github.com/dropezy/proto/v1/category"
	s_prpb "github.com/dropezy/proto/v1/product"
)

const serviceName = "product"

// Handler holds product gRPC service implementation.
type Handler struct {
	// utilities
	logger zerolog.Logger

	// service dependencies
	store storage.ProductStore
}

// NewHandler returns a new product service handler.
func NewHandler(logger zerolog.Logger, store storage.ProductStore) *Handler {
	return &Handler{
		logger: logger.With().Str("service", serviceName).Logger(),
		store:  store,
	}
}

// RegisterService registers the product service to the gRPC server.
func RegisterService(logger zerolog.Logger, store storage.ProductStore) func(srv *grpc.Server) error {
	return func(srv *grpc.Server) error {
		h := NewHandler(logger, store)
		prpb.RegisterProductServiceServer(srv, h)
		return nil
	}
}

func RegisterGateway(ctx context.Context, mux *runtime.ServeMux, addr string, opts []grpc.DialOption) error {
	return prpb.RegisterProductServiceHandlerFromEndpoint(ctx, mux, addr, opts)
}

// Get will fetch products from storage.
func (h *Handler) Get(ctx context.Context, req *prpb.GetRequest) (*prpb.GetResponse, error) {
	products, err := h.store.GetProducts(ctx)
	if err != nil {
		h.logger.Err(err).Msg("failed to fetch products from store")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &prpb.GetResponse{
		Products: toProductsPb(products),
	}, nil
}

func toProductsPb(products []*product.Product) []*s_prpb.Product {
	var productsPb []*s_prpb.Product
	for _, product := range products {
		productsPb = append(productsPb, &s_prpb.Product{
			ProductId:  product.ID.Hex(),
			Name:       product.Name_ID,
			ImagesUrls: product.ImagesURLs,
			Category_1: &s_ctpb.Category{
				CategoryId: product.Category1ID.Hex(),
			},
			Category_2: &s_ctpb.Category{
				CategoryId: product.Category2ID.Hex(),
			},
		})
	}
	return productsPb
}
