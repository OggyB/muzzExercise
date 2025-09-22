// internal/errors/mapper.go
package errors

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// Map converts repo/infra errors into gRPC-friendly status errors.
// Keeps service layer clean by centralizing error mapping.
func Map(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return status.Error(codes.NotFound, "record not found")

	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request timed out")

	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request was canceled")

	default:
		// fallback → bubble up error message for debugging
		return status.Error(codes.Internal, err.Error())
	}
}

// InvalidArgument creates a gRPC InvalidArgument error.
// Use this in service layer for bad input validation.
func InvalidArgument(msg string) error {
	return status.Error(codes.InvalidArgument, msg)
}

// AlreadyExists creates a gRPC AlreadyExists error.
func AlreadyExists(msg string) error {
	return status.Error(codes.AlreadyExists, msg)
}
