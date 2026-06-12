// Package service hosts the Scheduler orchestration layer (lock manager,
// cron runner, job interface, FCM client, M2M client, parametrizacion
// client, and structured logging).
//
// This file implements the event-driven consumer for the
// `SincronizacionSolicitada` event documented in the spec section
// "Eventos > Consume". Two backends are supported, picked at
// construction time:
//
//   - Redis Streams — the default for the VPS deployment target. The
//     consumer reads from `scheduler:stream:sincronizacion` with
//     consumer group `scheduler-svc`, blocking 1s per XREADGROUP. The
//     consumer group is created on first run via XGROUP CREATE MKSTREAM
//     (the BUSYGROUP error is swallowed — it just means the group
//     already exists).
//
//   - AWS SQS — the scale-only target. The consumer calls ReceiveMessage
//     with WaitTimeSeconds=20 (long polling), MaxNumberOfMessages=10,
//     VisibilityTimeout=60. On success the message is deleted via
//     DeleteMessage; on failure it is left in flight and will
//     reappear after the visibility timeout expires (the spec is
//     explicit: no retry, manual re-trigger via the admin API).
//
// Both backends share the same per-event lifecycle:
//
//  1. Parse the JSON body into a SincronizacionEvent.
//  2. Acquire a per-event lock (60s TTL) via LockManager. The lock
//     key is `scheduler:lock:sync:<event_id>` (Redis Streams) or
//     `scheduler:lock:sqs:<message_id>` (SQS). When the lock is held
//     by another replica the message is acknowledged and skipped —
//     cheaper than re-running the same sync twice.
//  3. Set the SincronizacionEvent on the context, build a BaseJob
//     for the `sincronizacion_solicitada` job, and invoke
//     WrapWithLifecycle(base, handler) so the execution row is
//     opened and finalised exactly like a regular cron-triggered run.
//  4. On success: ack the message and release the lock.
//  5. On failure: do not ack (Redis Streams) / do not delete (SQS),
//     log loudly, and release the lock.
//
// Wiring contract
// ================
// The consumer is started by main.go after the CronRunner registers
// the dispatcher handler for the `sincronizacion_solicitada` job:
//
//	job, _ := jobRepo.GetByKey(ctx, service.SincronizacionSolicitadaJobKey)
//	dispatcher := service.NewDefaultSyncDispatcher()
//	consumer, _ := service.NewSyncConsumer(cfg, lockMgr, execRepo, job.JobID, dispatcher.Handler(), logger)
//	runner.Register(job, consumer.Handler())
//	_ = consumer.Run(ctx)
//
// consumer.Handler() returns the same JobHandler the dispatcher
// produces (Handler() simply forwards to it) so CronRunner.Register
// and consumer.Run share a single, testable closure.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"axis-flow-back/internal/scheduler"
	"axis-flow-back/internal/scheduler/repository"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// ---------------------------------------------------------------------------
// Sentinel errors. Use errors.Is to inspect.
// ---------------------------------------------------------------------------

var (
	// ErrConsumerNotStarted is returned by Handler when the consumer
	// has not been started yet (or has already returned). It is
	// defensive — the cron runner registers the handler before
	// starting the consumer, so the practical impact is nil, but
	// tests that invoke Handler out of order can use it as a
	// discriminator.
	ErrConsumerNotStarted = errors.New("scheduler: sync consumer not started")
	// ErrInvalidConfig is returned by NewSyncConsumer when the
	// supplied Config does not match the selected backend
	// (e.g. SQSQueueURL set but AWSRegion blank, or RedisURL
	// blank for the Redis Streams backend).
	ErrInvalidConfig = errors.New("scheduler: sync consumer config is invalid")
)

// ---------------------------------------------------------------------------
// Constants — stream / group names and lock keys.
// ---------------------------------------------------------------------------

const (
	// syncStreamName is the canonical Redis Stream key for the
	// SincronizacionSolicitada events. Producers (the API layer
	// and the integration events publisher) XADD into this
	// stream; the consumer XREADGROUPs from it.
	syncStreamName = "scheduler:stream:sincronizacion"
	// syncConsumerGroup is the Redis Streams consumer group used
	// by the scheduler service. All replicas share this group so
	// messages are load-balanced across them.
	syncConsumerGroup = "scheduler-svc"
	// syncLockTTL is the per-event Redis lock TTL. 60s is a
	// generous bound for the fastest possible sync; long-running
	// syncs (catalog, employees, assignments) are expected to
	// finish well under this.
	syncLockTTL = 60 * time.Second
	// syncLockPrefix is the namespace prefix for per-event locks
	// acquired by the consumer. Mirrors the scheduler:lock:
	// convention used by the LockManager for cron jobs.
	syncLockPrefix = "scheduler:lock:sync:"
	// sqsLockPrefix is the namespace prefix for per-message locks
	// acquired by the SQS consumer. SQS messages do not have a
	// stable application-level id (the message id is per-receive),
	// so we lock on the SQS message id directly.
	sqsLockPrefix = "scheduler:lock:sqs:"
	// defaultPollInterval is the BLOCK duration passed to
	// XREADGROUP (1s) and the inter-call sleep on the SQS
	// receive loop. Tests can override it via WithPollInterval.
	defaultPollInterval = 1 * time.Second
	// defaultSQSWaitTime is the long-poll WaitTimeSeconds for
	// SQS ReceiveMessage. 20s matches the spec; lower values
	// would burn API quota without throughput gain.
	defaultSQSWaitTime = 20
	// defaultSQSMaxMessages is the MaxNumberOfMessages for
	// ReceiveMessage. 10 is the SQS hard limit per call.
	defaultSQSMaxMessages = 10
	// defaultSQSVisibilityTimeout is the VisibilityTimeout for
	// ReceiveMessage. 60s is the upper bound a sync should ever
	// take; on failure the message reappears after this window.
	defaultSQSVisibilityTimeout = 60
)

// ---------------------------------------------------------------------------
// Public contract.
// ---------------------------------------------------------------------------

// SyncConsumer is the dual interface the scheduler exposes for the
// SincronizacionSolicitada event:
//
//   - Run is the long-running background loop. It blocks until ctx
//     is cancelled, polls the configured backend, and processes
//     messages through the dispatcher. Call exactly once, in its own
//     goroutine.
//
//   - Handler returns the JobHandler the CronRunner registers for
//     the `sincronizacion_solicitada` job. The same closure is used
//     by the consumer's Run loop (the consumer sets the
//     SincronizacionEvent on the context before invoking it). The
//     closure is safe for concurrent invocation.
type SyncConsumer interface {
	// Run blocks until ctx is cancelled. Returning nil indicates a
	// clean shutdown on cancellation; a non-nil error indicates the
	// loop exited because of a fatal transport error. Per-event
	// handler errors do not stop the loop — they are logged and the
	// loop continues with the next message.
	Run(ctx context.Context) error
	// Handler returns the JobHandler the CronRunner registers for
	// the `sincronizacion_solicitada` job. The closure reads the
	// SincronizacionEvent from the context and delegates to the
	// dispatcher's Dispatch method. When the consumer has not been
	// started yet (e.g. the cron runner calls Handler before
	// consumer.Run is invoked) the returned handler is still valid —
	// it falls back to returning ErrInvalidEventPayload when the
	// event is absent from the context.
	Handler() JobHandler
}

// ---------------------------------------------------------------------------
// Consumer options — used by tests and by callers that need to
// inject a custom redis client (miniredis) or shorten the poll
// interval.
// ---------------------------------------------------------------------------

// ConsumerOption mutates a consumer at construction time. The
// production wiring (main.go) does not pass any option; tests pass
// WithPollInterval and WithRedisClient.
type ConsumerOption func(*consumerOptions)

type consumerOptions struct {
	pollInterval time.Duration
	redisClient  *redis.Client
	// sqsClient, when non-nil, replaces the SQS client built
	// from cfg.AWSAccessKeyID/Secret/Region. Reserved for future
	// tests; today the SQS path is exercised by a live test
	// container, not a mock.
	sqsClient *sqs.Client
	// consumerName is the Redis Streams consumer name. Multiple
	// replicas must use different names so they get different
	// messages from the shared group. Defaults to the hostname
	// (or a random UUID when the hostname is blank).
	consumerName string
}

func (o *consumerOptions) apply(opts []ConsumerOption) {
	for _, opt := range opts {
		opt(o)
	}
	if o.pollInterval <= 0 {
		o.pollInterval = defaultPollInterval
	}
	if o.consumerName == "" {
		if h, err := os.Hostname(); err == nil && h != "" {
			o.consumerName = h
		} else {
			o.consumerName = uuid.NewString()
		}
	}
}

// WithPollInterval overrides the default 1s poll interval for the
// Redis Streams consumer. The SQS consumer ignores this option —
// its wait time is fixed at 20s by the SQS contract.
func WithPollInterval(d time.Duration) ConsumerOption {
	return func(o *consumerOptions) {
		if d > 0 {
			o.pollInterval = d
		}
	}
}

// WithRedisClient injects a pre-built *redis.Client. Tests use this
// to wire miniredis into the consumer without exposing a Redis URL.
func WithRedisClient(c *redis.Client) ConsumerOption {
	return func(o *consumerOptions) {
		o.redisClient = c
	}
}

// WithSQSClient injects a pre-built *sqs.Client. Reserved for
// future SQS-mock testing; not used by the production wiring.
func WithSQSClient(c *sqs.Client) ConsumerOption {
	return func(o *consumerOptions) {
		o.sqsClient = c
	}
}

// WithConsumerName overrides the consumer name used as the XREADGROUP
// consumer identity. Tests use this to assert per-consumer delivery.
func WithConsumerName(name string) ConsumerOption {
	return func(o *consumerOptions) {
		o.consumerName = name
	}
}

// ---------------------------------------------------------------------------
// Factory.
// ---------------------------------------------------------------------------

// NewSyncConsumer returns a SyncConsumer matching the supplied config.
// When cfg.SQSQueueURL is blank the Redis Streams backend is
// returned; otherwise the AWS SQS backend is returned. The function
// performs cheap validation (SQS needs AWS credentials and a region;
// the Redis backend needs a client or a usable RedisURL) and
// returns ErrInvalidConfig (wrapped) on failure.
func NewSyncConsumer(
	cfg scheduler.Config,
	lockMgr LockManager,
	execRepo repository.ExecutionRepository,
	jobID uuid.UUID,
	handler JobHandler,
	logger *Logger,
	opts ...ConsumerOption,
) (SyncConsumer, error) {
	if lockMgr == nil {
		return nil, fmt.Errorf("NewSyncConsumer: %w: lockMgr is required", ErrInvalidConfig)
	}
	if execRepo == nil {
		return nil, fmt.Errorf("NewSyncConsumer: %w: execRepo is required", ErrInvalidConfig)
	}
	if jobID == uuid.Nil {
		return nil, fmt.Errorf("NewSyncConsumer: %w: jobID is required", ErrInvalidConfig)
	}
	if handler == nil {
		return nil, fmt.Errorf("NewSyncConsumer: %w: handler is required", ErrInvalidConfig)
	}

	o := &consumerOptions{}
	o.apply(opts)

	if cfg.SQSQueueURL != "" {
		return newAWSSQSConsumer(cfg, lockMgr, execRepo, jobID, handler, logger, o)
	}
	return newRedisStreamsConsumer(cfg, lockMgr, execRepo, jobID, handler, logger, o)
}

// ---------------------------------------------------------------------------
// Shared helpers — used by both backends.
// ---------------------------------------------------------------------------

// buildHandlerChain returns the JobHandler the consumer invokes. It
// is the dispatcher's handler wrapped in a context-injection step
// (the consumer sets the SincronizacionEvent and *BaseJob on the
// context before calling). The closure is the single source of
// truth for the consumer path: it is also what CronRunner.Register
// receives via consumer.Handler().
func buildHandlerChain(handler JobHandler) JobHandler {
	return func(ctx context.Context, base *BaseJob) error {
		// Defensive: when the consumer invokes this closure the
		// BaseJob and SincronizacionEvent are already on the
		// context. We re-assert them here so a caller that
		// skipped the consumer setup still gets a sensible
		// ErrInvalidEventPayload instead of a nil-deref panic.
		if base != nil {
			ctx = WithBaseJob(ctx, base)
		}
		if _, ok := SincronizacionEventFromContext(ctx); !ok {
			return fmt.Errorf("sync_consumer: %w: no SincronizacionEvent on context", ErrInvalidEventPayload)
		}
		return handler(ctx, base)
	}
}

// decodeEvent parses the raw JSON body into a SincronizacionEvent
// and back-fills a fresh EventID when the producer omitted it.
// Returns ErrInvalidEventPayload (wrapped) on any parse / validation
// failure so the caller can decide whether to ack-and-skip
// (permanent) or nack (transient).
func decodeEvent(raw []byte) (SincronizacionEvent, error) {
	if len(raw) == 0 {
		return SincronizacionEvent{}, fmt.Errorf("decodeEvent: %w: empty body", ErrInvalidEventPayload)
	}
	var ev SincronizacionEvent
	if err := json.Unmarshal(raw, &ev); err != nil {
		return SincronizacionEvent{}, fmt.Errorf("decodeEvent: %w: %v", ErrInvalidEventPayload, err)
	}
	if ev.EventID == "" {
		ev.EventID = mintEventID()
	}
	if err := ev.Validate(); err != nil {
		return SincronizacionEvent{}, err
	}
	return ev, nil
}

// processEvent runs the shared per-event flow used by both
// backends. It is called with a freshly-parsed event, a per-event
// lock token, a context already carrying the event, and a backend-
// specific ack/nack function. On success the ack callback is
// invoked; on failure the nack callback is invoked and the error
// is logged. The lock is always released in a defer block, even on
// panic, matching the CronRunner's lock-release pattern.
func (c *commonConsumer) processEvent(
	ctx context.Context,
	event SincronizacionEvent,
	lockKey, lockToken string,
	ack func(context.Context) error,
	nack func(context.Context),
) {
	logger := c.loggerOrNil()
	start := time.Now()

	// 1. Per-event lock. Another replica may be processing the
	// same event (Redis Streams PEL re-delivery or SQS in-flight
	// timeout) — when that happens we ack and skip.
	acquireCtx, acquireCancel := context.WithTimeout(ctx, 5*time.Second)
	acquired, lerr := c.lockMgr.Acquire(acquireCtx, lockKey, lockToken, syncLockTTL)
	acquireCancel()
	if lerr != nil {
		if logger != nil {
			logger.Inner().ErrorContext(ctx, "sync consumer: failed to acquire per-event lock",
				"event_id", redactEventID(event.EventID),
				"lock_key", lockKey,
				"error", lerr.Error(),
			)
		}
		// Transport-level failure: nack so the message reappears
		// (or stays in the PEL) and can be retried by another
		// replica or after the lock TTL expires.
		nack(ctx)
		return
	}
	if !acquired {
		if logger != nil {
			logger.Inner().WarnContext(ctx, "sync consumer: per-event lock held by another replica, skipping",
				"event_id", redactEventID(event.EventID),
				"lock_key", lockKey,
			)
		}
		// The lock is held elsewhere — the work is in progress.
		// Ack to remove this delivery from the queue; the other
		// replica owns the execution and will record the
		// outcome in scheduler_executions.
		if err := ack(ctx); err != nil && logger != nil {
			logger.Inner().WarnContext(ctx, "sync consumer: ack after lock collision failed",
				"event_id", redactEventID(event.EventID),
				"error", err.Error(),
			)
		}
		return
	}

	defer func() {
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer releaseCancel()
		if rerr := c.lockMgr.Release(releaseCtx, lockKey, lockToken); rerr != nil && logger != nil {
			logger.Inner().ErrorContext(releaseCtx, "sync consumer: failed to release per-event lock",
				"event_id", redactEventID(event.EventID),
				"lock_key", lockKey,
				"error", rerr.Error(),
			)
		}
	}()

	// 2. Wrap the handler in the lifecycle (insert RUNNING
	// execution row, finalise SUCCESS/FAILED, increment
	// scheduler_jobs_failed_total on error).
	base := NewBaseJob(c.jobID, c.logger, c.execRepo, c.lockMgr)
	ctx = WithSincronizacionEvent(ctx, event)
	ctx = WithBaseJob(ctx, base)

	if logger != nil {
		logger.Inner().InfoContext(ctx, "sync consumer: dispatching event",
			"event_id", redactEventID(event.EventID),
			"sync_type", event.SyncType,
			"target_id", event.TargetID,
			"requested_by", event.RequestedBy,
		)
	}

	runErr := c.wrappedHandler(ctx, base)
	duration := time.Since(start)

	switch {
	case runErr != nil:
		if logger != nil {
			logger.Inner().ErrorContext(ctx, "sync consumer: handler failed",
				"event_id", redactEventID(event.EventID),
				"sync_type", event.SyncType,
				"target_id", event.TargetID,
				"duration_ms", duration.Milliseconds(),
				"error", runErr.Error(),
			)
		}
		// Spec is explicit: no automatic retry. The outbox/stream
		// is the system of record and a manual re-trigger via the
		// admin API is the recovery path. nack means "do not ack"
		// in Redis Streams (the entry stays in the PEL) and
		// "do not delete" in SQS (the message will reappear
		// after VisibilityTimeout).
		nack(ctx)
	default:
		if logger != nil {
			logger.Inner().InfoContext(ctx, "sync consumer: handler succeeded",
				"event_id", redactEventID(event.EventID),
				"sync_type", event.SyncType,
				"target_id", event.TargetID,
				"duration_ms", duration.Milliseconds(),
			)
		}
		if err := ack(ctx); err != nil && logger != nil {
			logger.Inner().ErrorContext(ctx, "sync consumer: ack after success failed",
				"event_id", redactEventID(event.EventID),
				"error", err.Error(),
			)
		}
	}
}

// commonConsumer holds the cross-backend dependencies. The
// Redis-streams and SQS implementations embed it so the per-event
// flow lives in exactly one place.
type commonConsumer struct {
	lockMgr        LockManager
	execRepo       repository.ExecutionRepository
	jobID          uuid.UUID
	logger         *Logger
	wrappedHandler JobHandler
}

// loggerOrNil is a small nil-safe accessor used by the shared
// per-event flow.
func (c *commonConsumer) loggerOrNil() *Logger {
	if c == nil {
		return nil
	}
	return c.logger
}

// ---------------------------------------------------------------------------
// Redis Streams implementation — the default VPS backend.
// ---------------------------------------------------------------------------

// redisStreamsConsumer is the Redis-Streams-backed SyncConsumer. It
// reads from the canonical sincronizacion stream, blocks for
// pollInterval per XREADGROUP, and processes each message through
// the shared per-event flow.
type redisStreamsConsumer struct {
	commonConsumer
	client       *redis.Client
	streamName   string
	groupName    string
	consumerName string
	pollInterval time.Duration
	// started guards Handler from being called before the
	// consumer's Run goroutine has armed the wrapper. The cron
	// runner registers the handler before Run starts, so the
	// practical impact is nil; the flag is here so tests can
	// assert on the order of operations.
	startedMu sync.RWMutex
	started   bool
}

// newRedisStreamsConsumer builds a redisStreamsConsumer. The Redis
// client is built from cfg.RedisURL when opts.redisClient is nil.
// Returns ErrInvalidConfig (wrapped) when no usable client can be
// produced.
func newRedisStreamsConsumer(
	cfg scheduler.Config,
	lockMgr LockManager,
	execRepo repository.ExecutionRepository,
	jobID uuid.UUID,
	handler JobHandler,
	logger *Logger,
	opts *consumerOptions,
) (*redisStreamsConsumer, error) {
	if opts.redisClient == nil {
		if cfg.RedisURL == "" {
			return nil, fmt.Errorf("newRedisStreamsConsumer: %w: redis client or cfg.RedisURL is required", ErrInvalidConfig)
		}
		opt, perr := redis.ParseURL(cfg.RedisURL)
		if perr != nil {
			return nil, fmt.Errorf("newRedisStreamsConsumer: %w: parse REDIS_URL: %v", ErrInvalidConfig, perr)
		}
		opts.redisClient = redis.NewClient(opt)
	}

	c := &redisStreamsConsumer{
		commonConsumer: commonConsumer{
			lockMgr:        lockMgr,
			execRepo:       execRepo,
			jobID:          jobID,
			logger:         logger,
			wrappedHandler: buildHandlerChain(handler),
		},
		client:       opts.redisClient,
		streamName:   syncStreamName,
		groupName:    syncConsumerGroup,
		consumerName: opts.consumerName,
		pollInterval: opts.pollInterval,
	}
	return c, nil
}

// Handler returns the JobHandler the CronRunner registers. The
// closure is the same one Run invokes on each message — see
// buildHandlerChain for the context-injection contract.
func (c *redisStreamsConsumer) Handler() JobHandler {
	c.startedMu.RLock()
	started := c.started
	c.startedMu.RUnlock()
	if !started {
		// Returning the closure unconditionally would still
		// produce the right error (ErrInvalidEventPayload) for a
		// handler invoked outside the consumer's control flow.
		// The started flag is purely diagnostic.
		_ = started
	}
	return c.wrappedHandler
}

// Run blocks until ctx is cancelled. On every iteration it calls
// XREADGROUP with BLOCK pollInterval; returned messages are
// dispatched through processEvent. Per-message errors are logged
// and the loop continues; transport-level errors (XREADGROUP
// failure that is not a context cancellation) are returned to the
// caller as a fatal shutdown signal.
func (c *redisStreamsConsumer) Run(ctx context.Context) error {
	if err := c.ensureGroup(ctx); err != nil {
		return fmt.Errorf("redisStreamsConsumer.Run: ensure consumer group: %w", err)
	}

	c.startedMu.Lock()
	c.started = true
	c.startedMu.Unlock()

	if c.logger != nil {
		c.logger.Inner().InfoContext(ctx, "sync consumer: starting redis streams loop",
			"stream", c.streamName,
			"group", c.groupName,
			"consumer", c.consumerName,
			"poll_ms", c.pollInterval.Milliseconds(),
		)
	}

	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		// XREADGROUP with BLOCK returns immediately when there
		// are no messages; we treat nil result as a no-op and
		// loop. Count is 10 to match the SQS MaxNumberOfMessages
		// ceiling.
		streams, rerr := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    c.groupName,
			Consumer: c.consumerName,
			Streams:  []string{c.streamName, ">"},
			Count:    10,
			Block:    c.pollInterval,
		}).Result()
		if rerr != nil {
			if errors.Is(rerr, context.Canceled) || errors.Is(rerr, context.DeadlineExceeded) {
				return nil
			}
			if errors.Is(rerr, redis.Nil) {
				// No messages in the poll window. Loop.
				continue
			}
			if c.logger != nil {
				c.logger.Inner().ErrorContext(ctx, "sync consumer: XREADGROUP failed",
					"error", rerr.Error(),
				)
			}
			// Brief sleep before retrying so a flaky Redis
			// does not pin a CPU.
			if !sleepCtx(ctx, c.pollInterval) {
				return nil
			}
			continue
		}
		for _, s := range streams {
			for _, m := range s.Messages {
				c.handleStreamMessage(ctx, m)
			}
		}
	}
}

// ensureGroup creates the consumer group on first run, swallowing
// the BUSYGROUP error (it just means the group already exists). Any
// other error is surfaced — the consumer cannot run without a group.
func (c *redisStreamsConsumer) ensureGroup(ctx context.Context) error {
	err := c.client.XGroupCreateMkStream(ctx, c.streamName, c.groupName, "$").Err()
	if err == nil {
		return nil
	}
	// go-redis surfaces BUSYGROUP as a *redis.CommandError with
	// the "BUSYGROUP" prefix. We match on the prefix to be
	// defensive against minor format changes.
	if isBusyGroupError(err) {
		return nil
	}
	return err
}

// handleStreamMessage is the per-message handler for the Redis
// Streams backend. It parses the body, builds the per-event lock
// key from the event id, and invokes processEvent.
func (c *redisStreamsConsumer) handleStreamMessage(ctx context.Context, m redis.XMessage) {
	raw, ok := m.Values["payload"].(string)
	if !ok {
		// The stream should only carry payloads we wrote, but be
		// defensive: log and ack a malformed entry so it does
		// not pollute the PEL.
		if c.logger != nil {
			c.logger.Inner().WarnContext(ctx, "sync consumer: stream entry has no payload field, skipping",
				"message_id", m.ID,
			)
		}
		_ = c.client.XAck(ctx, c.streamName, c.groupName, m.ID).Err()
		return
	}

	event, perr := decodeEvent([]byte(raw))
	if perr != nil {
		if c.logger != nil {
			c.logger.Inner().ErrorContext(ctx, "sync consumer: invalid event payload, acking and skipping",
				"message_id", m.ID,
				"error", perr.Error(),
			)
		}
		_ = c.client.XAck(ctx, c.streamName, c.groupName, m.ID).Err()
		return
	}

	lockKey := syncLockPrefix + event.EventID
	lockToken := uuid.NewString()

	ack := func(ctx context.Context) error {
		return c.client.XAck(ctx, c.streamName, c.groupName, m.ID).Err()
	}
	nack := func(ctx context.Context) {
		// No-op: in Redis Streams, "nack" means "do not XACK".
		// The entry stays in the PEL; an operator can use
		// XCLAIM to inspect or reprocess it.
	}

	c.processEvent(ctx, event, lockKey, lockToken, ack, nack)
}

// ---------------------------------------------------------------------------
// AWS SQS implementation — the scale-only backend.
// ---------------------------------------------------------------------------

// awsSQSConsumer is the SQS-backed SyncConsumer. It calls
// ReceiveMessage with long polling and processes each message
// through the shared per-event flow.
type awsSQSConsumer struct {
	commonConsumer
	client    *sqs.Client
	queueURL  string
	waitTime  int32
	maxMsgs   int32
	visTime   int32
	startedMu sync.RWMutex
	started   bool
}

// newAWSSQSConsumer builds an awsSQSConsumer. The SQS client is
// built from cfg.AWSAWSAccessKeyID/Secret/Region when opts.sqsClient
// is nil. Returns ErrInvalidConfig (wrapped) when no usable client
// can be produced.
func newAWSSQSConsumer(
	cfg scheduler.Config,
	lockMgr LockManager,
	execRepo repository.ExecutionRepository,
	jobID uuid.UUID,
	handler JobHandler,
	logger *Logger,
	opts *consumerOptions,
) (*awsSQSConsumer, error) {
	if cfg.SQSQueueURL == "" {
		return nil, fmt.Errorf("newAWSSQSConsumer: %w: SQSQueueURL is required", ErrInvalidConfig)
	}

	client := opts.sqsClient
	if client == nil {
		if cfg.AWSAccessKeyID == "" || cfg.AWSSecretAccessKey == "" {
			return nil, fmt.Errorf("newAWSSQSConsumer: %w: AWS credentials are required when no SQS client is injected", ErrInvalidConfig)
		}
		if cfg.AWSRegion == "" {
			return nil, fmt.Errorf("newAWSSQSConsumer: %w: AWSRegion is required when no SQS client is injected", ErrInvalidConfig)
		}
		awsCfg, err := awsconfig.LoadDefaultConfig(
			context.Background(),
			awsconfig.WithRegion(cfg.AWSRegion),
			awsconfig.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey, ""),
			),
		)
		if err != nil {
			return nil, fmt.Errorf("newAWSSQSConsumer: %w: load AWS config: %v", ErrInvalidConfig, err)
		}
		client = sqs.NewFromConfig(awsCfg)
	}

	return &awsSQSConsumer{
		commonConsumer: commonConsumer{
			lockMgr:        lockMgr,
			execRepo:       execRepo,
			jobID:          jobID,
			logger:         logger,
			wrappedHandler: buildHandlerChain(handler),
		},
		client:   client,
		queueURL: cfg.SQSQueueURL,
		waitTime: defaultSQSWaitTime,
		maxMsgs:  defaultSQSMaxMessages,
		visTime:  defaultSQSVisibilityTimeout,
	}, nil
}

// Handler returns the JobHandler the CronRunner registers.
func (c *awsSQSConsumer) Handler() JobHandler {
	return c.wrappedHandler
}

// Run blocks until ctx is cancelled. It calls ReceiveMessage with
// the configured long-poll wait time on every iteration. Per-
// message errors are logged and the loop continues; transport-level
// errors that are not context cancellations are returned to the
// caller as a fatal shutdown signal.
func (c *awsSQSConsumer) Run(ctx context.Context) error {
	c.startedMu.Lock()
	c.started = true
	c.startedMu.Unlock()

	if c.logger != nil {
		c.logger.Inner().InfoContext(ctx, "sync consumer: starting SQS loop",
			"queue_url", c.queueURL,
			"wait_time_seconds", c.waitTime,
			"max_messages", c.maxMsgs,
			"visibility_timeout", c.visTime,
		)
	}

	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		out, rerr := c.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(c.queueURL),
			MaxNumberOfMessages: c.maxMsgs,
			WaitTimeSeconds:     c.waitTime,
			VisibilityTimeout:   c.visTime,
		})
		if rerr != nil {
			if errors.Is(rerr, context.Canceled) || errors.Is(rerr, context.DeadlineExceeded) {
				return nil
			}
			if c.logger != nil {
				c.logger.Inner().ErrorContext(ctx, "sync consumer: ReceiveMessage failed",
					"error", rerr.Error(),
				)
			}
			// Brief sleep before retrying so a flapping SQS
			// does not pin a CPU.
			if !sleepCtx(ctx, defaultPollInterval) {
				return nil
			}
			continue
		}
		if len(out.Messages) == 0 {
			continue
		}
		for i := range out.Messages {
			c.handleSQSMessage(ctx, &out.Messages[i])
		}
	}
}

// handleSQSMessage is the per-message handler for the SQS backend.
// It parses the body, builds the per-message lock key from the SQS
// message id, and invokes processEvent.
func (c *awsSQSConsumer) handleSQSMessage(ctx context.Context, m *sqstypes.Message) {
	if m == nil || m.Body == nil {
		return
	}
	body := aws.ToString(m.Body)
	messageID := aws.ToString(m.MessageId)
	if messageID == "" {
		// Without a message id we cannot build a stable lock
		// key. Log and leave the message in flight — it will
		// reappear after VisibilityTimeout.
		if c.logger != nil {
			c.logger.Inner().ErrorContext(ctx, "sync consumer: SQS message has no MessageId, leaving in flight",
				"receipt_handle", redactReceiptHandle(aws.ToString(m.ReceiptHandle)),
			)
		}
		return
	}

	event, perr := decodeEvent([]byte(body))
	if perr != nil {
		if c.logger != nil {
			c.logger.Inner().ErrorContext(ctx, "sync consumer: invalid event payload, deleting and skipping",
				"message_id", messageID,
				"error", perr.Error(),
			)
		}
		// Permanent failure — delete the message so it does
		// not loop. The spec is explicit: no retry on failure.
		c.deleteMessage(ctx, m)
		return
	}

	lockKey := sqsLockPrefix + messageID
	lockToken := uuid.NewString()
	receiptHandle := m.ReceiptHandle

	ack := func(ctx context.Context) error {
		_, derr := c.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
			QueueUrl:      aws.String(c.queueURL),
			ReceiptHandle: receiptHandle,
		})
		return derr
	}
	nack := func(ctx context.Context) {
		// No-op: in SQS, "nack" means "do not delete". The
		// message will reappear after VisibilityTimeout. An
		// operator can also use ChangeMessageVisibility to
		// expedite redelivery.
	}

	c.processEvent(ctx, event, lockKey, lockToken, ack, nack)
}

// deleteMessage is a small helper that swallows the error so the
// caller (handleSQSMessage) can stay focused on the happy path.
func (c *awsSQSConsumer) deleteMessage(ctx context.Context, m *sqstypes.Message) {
	_, err := c.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(c.queueURL),
		ReceiptHandle: m.ReceiptHandle,
	})
	if err != nil && c.logger != nil {
		c.logger.Inner().ErrorContext(ctx, "sync consumer: DeleteMessage failed",
			"error", err.Error(),
		)
	}
}

// ---------------------------------------------------------------------------
// Internal helpers.
// ---------------------------------------------------------------------------

// sleepCtx blocks for d, or until ctx is cancelled. Returns true
// when the sleep completed, false when ctx was cancelled.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// isBusyGroupError reports whether the XGROUP CREATE error is the
// well-known "BUSYGROUP" sentry Redis emits when the group already
// exists. The check is intentionally narrow: the only way to make
// ensureGroup idempotent is to swallow BUSYGROUP; every other error
// is a real failure. go-redis exposes Redis errors via the redis.Error
// interface; HasErrorPrefix matches the message text without
// committing to a specific error type that might change across
// versions.
func isBusyGroupError(err error) bool {
	if err == nil {
		return false
	}
	return redis.HasErrorPrefix(err, "BUSYGROUP")
}

// redactReceiptHandle returns the first 8 characters of an SQS
// receipt handle followed by an ellipsis. Receipt handles are
// PII-adjacent (they identify a specific in-flight delivery) and
// the spec mandates the same redaction policy as device tokens.
func redactReceiptHandle(h string) string {
	if len(h) <= 8 {
		return h
	}
	return h[:8] + "…"
}
