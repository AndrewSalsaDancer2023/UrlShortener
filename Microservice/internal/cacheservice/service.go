package cacheservice

import (
	"context"
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

/*
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
*/

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

func (s *UrlCacheService) WriteURLPair(ctx context.Context, short_url int64, long_url string) (int64, error) {
	shortKey := utils.CeateShortURLKey(short_url)
	hashedLongKey := utils.CreateHashedURLKey( /*short_url,*/ long_url)

	redisCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	//Ищем, не сокращали ли мы этот URL ранее
	existingShort, err := s.Client.Get(redisCtx, hashedLongKey).Int64() //Result()
	if err == nil {
		// Найдено! Возвращаем созданный ранее короткий URL
		return existingShort, nil
	}

	// ЗАПИСЬ КОРОТКОЙ ССЫЛКИ: Используем SetNX
	// Он вернет true, если ключ успешно создан, и false, если такой shortCode уже кем-то занят
	success, err := s.Client.SetNX(redisCtx, shortKey, long_url, 24*time.Hour).Result()
	if err != nil {
		return 0, status.Error(codes.Internal, "error happened during short_url : long_url pair saving")
	}

	if !success {
		return short_url, status.Error(codes.AlreadyExists, "short code collision, please retry")
	}

	// Так как короткий код успешно забит за нами, связываем хэш с этим кодом
	err = s.Client.Set(redisCtx, hashedLongKey, short_url, 24*time.Hour).Err()
	if err != nil {
		/*		go func() {
				cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer cleanupCancel()

				// Скрипт удалит shortToLongKey только если его значение равно longURL
				_, _ = safeDeleteScript.Run(cleanupCtx, s.Client, []string{shortKey}, []string{long_url}).Result()
			}()*/
		go DeleteURLPair(s.Client, shortKey, long_url)
		return 0, status.Error(codes.Internal, "error happened during long_url : short_url pair saving")
	}

	return short_url, nil
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
	hashedLongKey := utils.CreateHashedURLKey( /*short_url,*/ long_url)

	short_url, err := s.Client.Get(ctx, hashedLongKey).Int64()
	if err != nil {
		return 0, fmt.Errorf("unexpected redis error: %v", err)
	}
	return short_url, nil
}
