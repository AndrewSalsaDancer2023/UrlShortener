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
			// // ВАРИАНТ А: Автоматически подставляем безопасный таймаут по умолчанию (Рекомендуется)
			// var cancel context.CancelFunc
			// ctx, cancel = context.WithTimeout(ctx, defaultTimeout)
			// defer cancel()

			// ВАРИАНТ Б: Если вы хотите быть строгими и сразу «отшивать» клиента, раскомментируйте код ниже:
			return nil, status.Error(codes.InvalidArgument, "gRPC deadline must be specified by the client")
		}

		// 2. Передаем защищенный контекст дальше в ваш реальный обработчик сервиса
		return handler(ctx, req)
	}
}
