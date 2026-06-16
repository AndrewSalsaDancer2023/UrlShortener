package cacheservice

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"
	"urlshortener/internal/cacheservice/config"
	"urlshortener/utils"

	redis "github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UrlCache interface {
	TryConnect() error
	WriteURLPair(ctx context.Context, shortURL int64, longURL string) (int64, error)
	GetLongURL(ctx context.Context, shortURL int64) (string, error)
	GetShortURL(ctx context.Context, shortURL int64, longURL string) (int64, error)
	Ping(ctx context.Context) error
	Close() error
}

type UrlCacheService struct {
	client *redis.Client
	ttl    time.Duration
}

func CreateService(cfg *config.CacheConfig) *redis.Client {
	numCPU := runtime.NumCPU()
	return redis.NewClient(&redis.Options{
		Addr:            cfg.Host + ":" + cfg.RedisPort, // Адрес вашего Redis-сервера
		Password:        "",                             // Пароль (оставьте пустым, если не задан)
		DB:              0,                              // Номер используемой базы данных
		MinIdleConns:    cfg.MinIdleConnsMult * numCPU,
		MaxIdleConns:    cfg.MaxIdleConnsMult * numCPU,
		ConnMaxIdleTime: cfg.ConnMaxIdleTime,
	})
}

func New(cfg *config.CacheConfig) UrlCache {

	return &UrlCacheService{
		client: CreateService(cfg),
		ttl:    cfg.DataTTL,
	}
}

func (s *UrlCacheService) Ping(ctx context.Context) error {
	err := s.client.Ping(ctx).Err()
	if err != nil {
		return err
	}

	return nil
}

func (s *UrlCacheService) Close() error {
	return s.client.Close()
}

func (s *UrlCacheService) TryConnect() error {
	// Создаем контекст с таймаутом в 5 секунд на базе пустого Background-контекста
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	// ОБЯЗАТЕЛЬНО: всегда вызывайте cancel через defer!
	defer cancel()
	return s.Ping(ctx)
}

var safeDeleteScript = redis.NewScript(`
	if redis.call("GET", KEYS[1]) == ARGV[1] then
		return redis.call("DEL", KEYS[1])
	end
	return 0
`)

func DeleteURLPair(client *redis.Client, shortKey string, longURL string) {
	// Передаем context.Background(), чтобы отмена redisCtx не сорвала удаление
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cleanupCancel()

	// Скрипт удалит shortKey только если его значение равно longURL
	_, _ = safeDeleteScript.Run(cleanupCtx, client, []string{shortKey}, []string{longURL}).Result()
}

func (s *UrlCacheService) WriteURLPair(ctx context.Context, shortURL int64, longURL string) (int64, error) {
	shortKey := utils.CeateShortURLKey(shortURL)
	hashedLongKey := utils.CreateHashedURLKey(longURL)

	//Ищем, не сокращали ли мы этот URL ранее
	existingShort, err := s.client.Get(ctx, hashedLongKey).Int64() //Result()
	if err == nil {
		// Найдено! Возвращаем созданный ранее короткий URL
		return existingShort, nil
	}
	// ЗАПИСЬ КОРОТКОЙ ССЫЛКИ: Используем SetNX
	// Он вернет true, если ключ успешно создан, и false, если такой shortCode уже кем-то занят
	// success, err := s.client.SetNX(ctx, shortKey, longURL, s.ttl).Result()
	success, err := s.client.SetNX(ctx, hashedLongKey, shortURL, s.ttl).Result()
	if err != nil {
		return 0, fmt.Errorf("error happened during short_url : long_url pair saving: %v", err)
	}

	if !success {
		return 0, fmt.Errorf("source URL : %s already added", longURL)
	}

	// Так как короткий код успешно забит за нами, связываем хэш с этим кодом
	// err = s.client.Set(ctx, hashedLongKey, shortURL, s.ttl).Err()
	err = s.client.Set(ctx, shortKey, longURL, s.ttl).Err()
	if err != nil {
		DeleteURLPair(s.client, shortKey, longURL)
		return 0, status.Error(codes.Internal, "error happened during long_url : short_url pair saving")
	}

	return shortURL, nil
}

func (s *UrlCacheService) GetLongURL(ctx context.Context, shortURL int64) (string, error) {
	shortKey := utils.CeateShortURLKey(shortURL)
	longURL, err := s.client.Get(ctx, shortKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			//ключ hashedLongKey отсутствует в кэше
			return "", nil
		}
		return "", fmt.Errorf("unexpected redis error: %v", err)
	}

	return longURL, nil
}

func (s *UrlCacheService) GetShortURL(ctx context.Context, shortURL int64, longURL string) (int64, error) {
	hashedLongKey := utils.CreateHashedURLKey(longURL)

	cacheURL, err := s.client.Get(ctx, hashedLongKey).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			//ключ hashedLongKey отсутствует в кэше
			return 0, nil
		}
		return 0, fmt.Errorf("unexpected redis cache error: %v", err)
	}
	return cacheURL, nil
}
