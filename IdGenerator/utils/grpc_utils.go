package utils

import (
	"context"
	"log"
	"runtime/debug"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	LoggerOpts = []logging.Option{
		logging.WithLogOnEvents(logging.StartCall, logging.FinishCall),
	}
	grpcLogger = logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		log.Printf("[%s] %s %v", CreateDebugLevelString(lvl), msg, fields)
	})

	//4. Настройка Recover Interceptor (перехват panic)
	RecoveryOpts = []recovery.Option{
		recovery.WithRecoveryHandler(func(p any) (err error) {
			log.Printf("panic happened: %v\n Call stack:\n%s", p, string(debug.Stack()))
			return status.Errorf(codes.Internal, "Internal server error")
		}),
	}
)

func CreateGRPCServer(logOpts []logging.Option, recOpts []recovery.Option) *grpc.Server {
	return grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			recovery.UnaryServerInterceptor(recOpts...),
			logging.UnaryServerInterceptor(grpcLogger, logOpts...),
		),
	)
}
