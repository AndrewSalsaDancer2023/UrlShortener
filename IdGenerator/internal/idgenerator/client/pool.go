package idgenerator

// локальный пул с предвыборкой батчей.
import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc"

	"urlshortener/internal/idgenerator"
	pb "urlshortener/internal/proto/idservice"
)

// BatchFetcher —  интерфейс, нужный пулу ID.
// Тесты подставляют фейковую реализацию, не поднимая настоящее gRPC-соединение.
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
// Параметр n намеренно игнорируется: реальный размер батча определяет
// сервис (см. GetBatchResponse.Ids), клиент на него не влияет.
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

// Создаёт пул и сразу запускает фоновую предвыборку.
// lookaheadBatches — глубина локальной очереди в батчах.
// lookaheadBatches = 1 уже даёт скрытие сетевой задержки за счёт предвыборки
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

// Останавливает фоновую предвыборку. Уже полученные (в том числе
// предвыбранные, но ещё не розданные) id по-прежнему можно забрать через
// NextID, пока локальный запас не иссякнет. Новых батчей запрошено не будет.
func (p *Pool) Close() {
	p.cancel()
}

// Отдаёт очередной уникальный id.
// Если в текущем локальном батче есть запас, то возвращает id мгновенно, без
// сетевого вызова. Если локальный батч исчерпан, блокируется на
// buf.TakeBatch: в типичном случае там уже лежит предвыбранный батч
// Безопасен для конкурентного вызова из нескольких горутин: mutex
// удерживается на всё время получени батча намеренно.
// Это гарантирует, что при одновременном исчерпании батч запрашивается
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
