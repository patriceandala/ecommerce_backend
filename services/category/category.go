// Package category implements category gRPC service methods
// to manage dropezy product categories operations.
package category

import (
	"context"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dropezy/storefront-backend/internal/storage"
	"github.com/dropezy/storefront-backend/internal/storage/model/category"

	// protobuf
	ctpb "github.com/dropezy/proto/ems/v1/category"

	// old protobuf
	s_ctpb "github.com/dropezy/proto/v1/category"
)

const serviceName = "category"

// Handler holds category gRPC service implementation.
type Handler struct {
	// utilities
	logger zerolog.Logger

	// service dependencies
	store storage.CategoryStore
}

// NewHandler returns a new category service handler.
func NewHandler(logger zerolog.Logger, store storage.CategoryStore) *Handler {
	return &Handler{
		logger: logger.With().Str("service", serviceName).Logger(),
		store:  store,
	}
}

// RegisterService registers the category service to the gRPC server.
func RegisterService(logger zerolog.Logger, store storage.CategoryStore) func(srv *grpc.Server) error {
	return func(srv *grpc.Server) error {
		h := NewHandler(logger, store)
		ctpb.RegisterCategoryServiceServer(srv, h)
		return nil
	}
}

func RegisterGateway(ctx context.Context, mux *runtime.ServeMux, addr string, opts []grpc.DialOption) error {
	return ctpb.RegisterCategoryServiceHandlerFromEndpoint(ctx, mux, addr, opts)
}

// Get will fetch product categories from storage.
func (h *Handler) Get(ctx context.Context, req *ctpb.GetRequest) (*ctpb.GetResponse, error) {
	categories, err := h.store.GetCategories(ctx)
	if err != nil {
		h.logger.Err(err).Msg("failed to fetch categories from store")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &ctpb.GetResponse{
		Categories: toCategoriesPB(categories),
	}, nil
}

func toCategoriesPB(categories []*category.Category) []*s_ctpb.Category {
	var categoriesPB []*s_ctpb.Category

	for _, category := range categories {
		l2Categories := category.ChildCategories

		// map level 2 categories for this c1 category
		var l2CategoriesPB []*s_ctpb.Category
		for _, l2Category := range l2Categories {
			l2CategoriesPB = append(l2CategoriesPB, &s_ctpb.Category{
				CategoryId: l2Category.ID.Hex(),
				Level:      l2Category.Level,
				Name:       l2Category.Name_ID,
				ImagesUrls: l2Category.ImagesURLs,
			})
		}

		l1CategoryPB := &s_ctpb.Category{
			CategoryId:      category.ID.Hex(),
			Level:           category.Level,
			Name:            category.Name_ID,
			ImagesUrls:      category.ImagesURLs,
			ChildCategories: l2CategoriesPB,
		}

		categoriesPB = append(categoriesPB, l1CategoryPB)
	}
	return categoriesPB
}
