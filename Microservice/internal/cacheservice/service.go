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
)

type UrlCache interface {
	WriteURLPair(ctx context.Context, short_url int64, long_url string) (int64, error)
	GetLongURL(ctx context.Context, short_url int64) (string, error)
	GetShortURL(ctx context.Context, short_url int64, long_url string) (int64, error)
}

type UrlCacheService struct {
	Client *redis.Client
}

func CreateService(cfg *config.CacheConfig) *redis.Client {
	numCPU := runtime.NumCPU()
	return redis.NewClient(&redis.Options{
		Addr:            cfg.Host + ":" + cfg.Port, // Адрес вашего Redis-сервера
		Password:        "",                        // Пароль (оставьте пустым, если не задан)
		DB:              0,                         // Номер используемой базы данных
		MinIdleConns:    cfg.MinIdleConnsMult * numCPU,
		MaxIdleConns:    cfg.MaxIdleConnsMult * numCPU,
		ConnMaxIdleTime: cfg.ConnMaxIdleTime,
	})
}

func New(cfg *config.CacheConfig) *UrlCacheService {

	return &UrlCacheService{
		Client: CreateService(cfg),
	}
}

func (s *UrlCacheService) Ping(ctx context.Context) error {
	err := s.Client.Ping(ctx).Err()
	if err != nil {
		return err
	}

	return nil
}

func (s *UrlCacheService) WriteURLPair(ctx context.Context, short_url int64, long_url string) (int64, error) {
	shortKey := utils.CeateShortURLKey(short_url)
	hashedLongKey := utils.CreateHashedURLKey(short_url, long_url)

	var txGetOrCreateScript = redis.NewScript(`
	-- 1. Проверяем существование ключа
	local existing_url = redis.call("GET", KEYS[1])
	if existing_url then
		return existing_url -- Ключ найден: возвращаем текущий длинный URL
	end

	-- 2. Ключа нет: атомарно записываем обе пары с TTL с помощью универсальной команды SET
	-- Аргументы "PX" и ARGV[3] задают TTL в миллисекундах
	-- Аргумент "NX" гарантирует, что мы не перезапишем ключ, если в микросекунду между GET и SET что-то изменилось
	redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[3], "NX")
	redis.call("SET", KEYS[2], ARGV[2], "PX", ARGV[3], "NX")

	return nil -- Успешно создано
	`)

	keys := []string{hashedLongKey, shortKey}
	args := []interface{}{short_url, long_url, (24 * time.Hour).Milliseconds()}

	// Выполняем скрипт
	res, err := txGetOrCreateScript.Run(ctx, s.Client, keys, args...).Int64()

	if err != nil {
		// Если скрипт вернул nil (запись успешно создана), go-redis отдаст ошибку redis.Nil
		if errors.Is(err, redis.Nil) {
			return short_url, nil // Успешно создано, старого URL не было
		}
		// Любая другая сетевая или синтаксическая ошибка
		return 0, fmt.Errorf("lua script failed: %w", err)
	}

	// Если ошибки redis.Nil не было, значит скрипт вернул строку (существующий URL)
	// existingURL, ok := res.(string)
	// if !ok {
	// 	return 0, fmt.Errorf("unexpected return type from lua script: %T", res)
	// }

	return res, nil
}

func (s *UrlCacheService) GetLongURL(ctx context.Context, short_url int64) (string, error) {
	shortKey := utils.CeateShortURLKey(short_url)
	long_url, err := s.Client.Get(ctx, shortKey).Result()
	if err != nil {
		return "", fmt.Errorf("unexpected redis error: %v", err)
	}

	return long_url, nil
}

func (s *UrlCacheService) GetShortURL(ctx context.Context, short_url int64, long_url string) (int64, error) {
	hashedLongKey := utils.CreateHashedURLKey(short_url, long_url)

	short_url, err := s.Client.Get(ctx, hashedLongKey).Int64()
	if err != nil {
		return 0, fmt.Errorf("unexpected redis error: %v", err)
	}

	return short_url, nil
}
