package idgenerator

// Package client — клиентская библиотека для сервиса генерации ID:
// локальный пул с предвыборкой (prefetch) батчей.
//
// Переиспользует те же buffer.Buffer и producer.Producer, что и сам
// сервис, просто "на уровень выше": здесь Generator — не atomic-счётчик,
// а gRPC-вызов GetBatch к микросервису. Producer в фоне поддерживает
// buf заполненным на lookahead батчей вперёд, поэтому в типичном режиме
// NextID почти всегда отдаёт id из уже готового локального среза, не
// дожидаясь сетевого round-trip к сервису.
//
// Ключевой эффект, который делает это возможным даже при lookahead=1:
// Producer.Run генерирует (здесь — запрашивает по сети) следующий батч
// ДО того, как текущий слот буфера освобождён — то есть fetch следующего
// батча стартует, пока клиент ещё доедает текущий, а не после того, как
// он его исчерпал.

import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc"

	"urlshortener/internal/idgenerator"
	pb "urlshortener/internal/proto/idservice"
)

// BatchFetcher — минимальный срез сгенерированного pb.IDGeneratorClient,
// нужный Pool. Объявлен здесь, а не как прямая зависимость от
// pb.IDGeneratorClient — по тому же правилу "принимай интерфейсы,
// возвращай структуры", что и producer.Generator/server.BatchSource:
// тесты подставляют фейк, не поднимая настоящее gRPC-соединение.
type BatchFetcher interface {
	GetIDBatch(ctx context.Context, in *pb.GetBatchRequest, opts ...grpc.CallOption) (*pb.GetBatchResponse, error)
}

// rpcGenerator адаптирует BatchFetcher под интерфейс producer.Generator,
// чтобы переиспользовать существующий Producer для предвыборки по сети.
type rpcGenerator struct {
	client  BatchFetcher
	ctx     context.Context
	timeout time.Duration
}

// NextBatch запрашивает у сервиса очередной батч по gRPC.
//
// Параметр n намеренно игнорируется: реальный размер батча определяет
// сервис (см. GetBatchResponse.Ids), клиент на него не влияет. Это
// осознанное отступление от общего контракта producer.Generator ("ровно
// n элементов или ошибка") — здесь оно безопасно, потому что и Buffer,
// и Producer работают с батчем как с непрозрачным срезом произвольной
// длины и не проверяют его размер.
func (g *rpcGenerator) NextBatch(_ int) (idgenerator.IDBatch, error) {
	ctx, cancel := context.WithTimeout(g.ctx, g.timeout)
	defer cancel()

	resp, err := g.client.GetIDBatch(ctx, &pb.GetBatchRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Ids, nil
}

// Pool — клиентский локальный пул ID с предвыборкой.
type Pool struct {
	buf idgenerator.IDBuffer

	mu      sync.Mutex
	current idgenerator.IDBatch
	idx     int

	cancel context.CancelFunc
}

// NewPool создаёт пул и сразу запускает фоновую предвыборку.
//
// lookaheadBatches — сколько батчей держать готовыми вперёд (глубина
// локальной очереди в батчах). Минимум 1 уже даёт частичное скрытие
// сетевой задержки за счёт эффекта, описанного в комментарии к пакету;
// 2 — безопасный дефолт с запасом на джиттер сети. Увеличивать сильно
// дальше обычно не имеет смысла: выигрыш в задержке не растёт, а число
// id, "сгорающих" впустую при падении клиента с непотраченным запасом,
// растёт линейно.
//
// rpcTimeout — таймаут на один вызов GetBatch к сервису.
func NewPool(ctx context.Context, client BatchFetcher, lookaheadBatches int, rpcTimeout time.Duration, wg *sync.WaitGroup) *Pool {
	if lookaheadBatches < 1 {
		lookaheadBatches = 1
	}

	buf := idgenerator.NewBuffer(lookaheadBatches)

	cancelCtx, cancel := context.WithCancel(ctx)
	gen := &rpcGenerator{client: client, ctx: cancelCtx, timeout: rpcTimeout}
	prod := idgenerator.NewProducer(gen, buf, 0) // batchSize не используется rpcGenerator'ом — реальный размер решает сервис

	wg.Add(1)
	go func() {
		defer wg.Done() // Сообщаем серверу, что горутина продюсера полностью завершилась
		prod.Run(cancelCtx)
	}()

	return &Pool{buf: buf, cancel: cancel}
}

// Close останавливает фоновую предвыборку. Уже полученные (в том числе
// предвыбранные, но ещё не розданные) id по-прежнему можно забрать через
// NextID, пока локальный запас не иссякнет — новых батчей запрошено не будет.
func (p *Pool) Close() {
	p.cancel()
}

// NextID отдаёт очередной уникальный id.
//
// Если в текущем локальном батче есть запас — возвращает мгновенно, без
// сетевого вызова. Если локальный батч исчерпан, блокируется на
// buf.TakeBatch: в типичном случае там уже лежит предвыбранный батч
// (мгновенный ответ), в худшем — по-настоящему ждёт сеть.
//
// Безопасен для конкурентного вызова из нескольких горутин: mutex
// удерживается на всё время дозаправки локального батча намеренно —
// это гарантирует, что при одновременном исчерпании батч запрашивается
// только один раз, а не по разу на каждую заблокированную горутину.
func (p *Pool) NextID(ctx context.Context) (int64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for p.idx >= len(p.current) {
		batch, err := p.buf.TakeBatch(ctx)
		if err != nil {
			return 0, err
		}
		p.current = batch
		p.idx = 0
	}

	id := p.current[p.idx]
	p.idx++
	return id, nil
}
