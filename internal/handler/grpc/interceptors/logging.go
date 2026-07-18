package interceptors

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func LoggingUnaryInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	start := time.Now()

	resp, err := handler(ctx, req)

	duration := time.Since(start)
	code := status.Code(err)

	log.Printf(
		"grpc method=%s code=%s duration=%s",
		info.FullMethod,
		code.String(),
		duration.String(),
	)

	return resp, err
}
