package explore

import (
	"google.golang.org/grpc"

	"github.com/oggyb/muzz-exercise/internal/app"
	pb "github.com/oggyb/muzz-exercise/internal/proto/explore"
)

// Registrar ties the Explore service into the gRPC server
type Registrar struct {
	appCtx *app.AppContext
}

// NewRegistrar creates a new Registrar for the Explore service
func NewRegistrar(appCtx *app.AppContext) *Registrar {
	return &Registrar{appCtx: appCtx}
}

// Register attaches the Explore service implementation to the gRPC server
func (r *Registrar) Register(s *grpc.Server) {
	service := NewExploreService(r.appCtx)
	pb.RegisterExploreServiceServer(s, service)
}
