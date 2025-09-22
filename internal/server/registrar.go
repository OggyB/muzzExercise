package server

import "google.golang.org/grpc"

// Registrar is a common interface for all gRPC service registrars
type Registrar interface {
	Register(s *grpc.Server)
}
