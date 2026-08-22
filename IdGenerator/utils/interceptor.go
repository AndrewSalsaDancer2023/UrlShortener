package utils

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func EnforceDeadlineInterceptor( /*defaultTimeout time.Duration*/ ) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {

		// 1. Проверяем, задан ли уже дедлайн клиентом во входящем контексте
		_, hasDeadline := ctx.Deadline()

		if !hasDeadline {
			//  отклоняем запрос клиента, влзвращая ошибку
			return nil, status.Error(codes.InvalidArgument, "gRPC deadline must be specified by the client")
		}

		// 2. Передаем контекст дальше в обработчик сервиса
		return handler(ctx, req)
	}
}
