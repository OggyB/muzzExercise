package server

import (
	"fmt"
	"github.com/oggyb/muzz-exercise/internal/config"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// StartGRPCServer boots a gRPC server and registers all provided services
func StartGRPCServer(cfg *config.Config, registrars ...Registrar) error {
	addr := fmt.Sprintf("%s:%s", cfg.GRPC.Host, cfg.GRPC.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	grpcServer := grpc.NewServer()

	// register all services
	for _, r := range registrars {
		r.Register(grpcServer)
	}

	// enable reflection for easier debugging with grpcurl
	reflection.Register(grpcServer)

	return grpcServer.Serve(lis)
}
