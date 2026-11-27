package cron

import (
	"context"
	"errors"
	"hash/fnv"
	"log/slog"
	"sync"
	"time"

	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/syncstatus"

	"github.com/go-co-op/gocron/v2"
)

// EdgeSyncScheduler runs the edge sync loops on timers: the push loop drains this node's
// unsynced outbox to the cloud, the pull loop applies the cloud's reference changes, and the
// stock refresh adopts the cloud's on-hand once a day. It owns its own gocron scheduler so
// the edge sync concern is started and stopped independently of the cloud cron jobs.
type EdgeSyncScheduler struct {
	scheduler        gocron.Scheduler
	organizations    *service.OrganizationRunner
	boxOrgID         string
	pushService      *service.SyncPushService
	pullService      *service.SyncPullService
	stockPullService *service.StockPullService
	tracker          *syncstatus.Tracker
	pushCron         string
	pullCron         string
	stockPullCron    string
	logger           *slog.Logger
	nodeID           string
	now              func() time.Time
	// callMu serializes the loops' cloud calls, so after a pause only the first one probes the cloud.
	callMu   sync.Mutex
	stop     chan struct{}
	stopOnce sync.Once
}

const (
	defaultRefusalPause = 6 * time.Hour
	minRefusalPause     = time.Minute
	maxRefusalPause     = 24 * time.Hour
	minuteLoopSpread    = 45 * time.Second
	dailyLoopSpread     = 15 * time.Minute
)

type EdgeSyncOption func(*EdgeSyncScheduler)

// WithNodeID spreads this box's sync calls by an offset derived from its node id.
func WithNodeID(nodeID string) EdgeSyncOption {
	return func(s *EdgeSyncScheduler) { s.nodeID = nodeID }
}

func WithClock(now func() time.Time) EdgeSyncOption {
	return func(s *EdgeSyncScheduler) { s.now = now }
}

// NewEdgeSyncScheduler runs every loop as the box's organization, boxOrgID.
func NewEdgeSyncScheduler(
	organizations *service.OrganizationRunner,
	boxOrgID string,
	pushService *service.SyncPushService,
	pullService *service.SyncPullService,
	stockPullService *service.StockPullService,
	tracker *syncstatus.Tracker,
	pushCron string,
	pullCron string,
	stockPullCron string,
	logger *slog.Logger,
	opts ...EdgeSyncOption,
) (*EdgeSyncScheduler, error) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}

	s := &EdgeSyncScheduler{
		scheduler:        scheduler,
		organizations:    organizations,
		boxOrgID:         boxOrgID,
		pushService:      pushService,
		pullService:      pullService,
		stockPullService: stockPullService,
		tracker:          tracker,
		pushCron:         pushCron,
		pullCron:         pullCron,
		stockPullCron:    stockPullCron,
		logger:           logger,
		now:              time.Now,
		stop:             make(chan struct{}),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

func (s *EdgeSyncScheduler) Start() error {
	if err := s.registerJobs(); err != nil {
		return err
	}

	s.scheduler.Start()
	s.logger.Info("Edge sync scheduler started",
		slog.String("push_cron", s.pushCron),
		slog.String("pull_cron", s.pullCron),
		slog.String("stock_pull_cron", s.stockPullCron),
		slog.Duration("minute_offset", s.offset(minuteLoopSpread)),
		slog.Duration("daily_offset", s.offset(dailyLoopSpread)),
	)
	return nil
}

func (s *EdgeSyncScheduler) registerJobs() error {
	if _, err := s.scheduler.NewJob(
		gocron.CronJob(s.pushCron, false),
		gocron.NewTask(s.spread(minuteLoopSpread, s.pushJob)),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	); err != nil {
		s.logger.Error("Failed to register sync push cron job", slog.Any("error", err))
		return err
	}

	if _, err := s.scheduler.NewJob(
		gocron.CronJob(s.pullCron, false),
		gocron.NewTask(s.spread(minuteLoopSpread, s.pullJob)),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	); err != nil {
		s.logger.Error("Failed to register sync pull cron job", slog.Any("error", err))
		return err
	}

	if _, err := s.scheduler.NewJob(
		gocron.CronJob(s.stockPullCron, false),
		gocron.NewTask(s.spread(dailyLoopSpread, s.stockPullJob)),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	); err != nil {
		s.logger.Error("Failed to register stock pull cron job", slog.Any("error", err))
		return err
	}

	return nil
}

// stockPullJob adopts the cloud's on-hand for the day. Only a refusal reaches the tracker:
// unlike the reference pull it runs once a day, so its silence says nothing about whether
// the cloud is reachable right now. A failure leaves the previous
// amounts in place and touches nothing else — selling never depends on this.
func (s *EdgeSyncScheduler) stockPullJob() {
	s.whenNotPaused(func() { s.asBox(s.stockPull) })
}

func (s *EdgeSyncScheduler) stockPull(ctx context.Context) {
	result, err := s.stockPullService.RefreshStock(ctx)
	if err != nil {
		s.recordRefusal(err)
		s.logger.ErrorContext(ctx, "Edge stock refresh job failed", slog.Any("error", err))
		return
	}
	s.logger.InfoContext(ctx, "Edge stock refresh job completed",
		slog.Int("stock_rows", result.Stock),
	)
}

// asBox runs job for the box's organization, in a database session of its own.
func (s *EdgeSyncScheduler) asBox(job func(ctx context.Context)) {
	_ = s.organizations.Run(context.Background(), s.boxOrgID, func(ctx context.Context) error {
		job(ctx)
		return nil
	})
}

func (s *EdgeSyncScheduler) Stop() error {
	s.logger.Info("Stopping edge sync scheduler...")
	s.stopOnce.Do(func() { close(s.stop) })
	return s.scheduler.Shutdown()
}

// offset is stable per node, so restarts keep it and different boxes don't all call at second 0.
func (s *EdgeSyncScheduler) offset(spread time.Duration) time.Duration {
	if s.nodeID == "" {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(s.nodeID))
	return time.Duration(int64(h.Sum32())%spread.Milliseconds()) * time.Millisecond
}

// spread waits this box's offset before job, giving up if the scheduler stops meanwhile.
func (s *EdgeSyncScheduler) spread(within time.Duration, job func()) func() {
	return func() {
		if wait := s.offset(within); wait > 0 {
			timer := time.NewTimer(wait)
			defer timer.Stop()
			select {
			case <-s.stop:
				return
			case <-timer.C:
			}
		}
		job()
	}
}

// whenNotPaused runs call unless a cloud refusal paused sync and its delay hasn't passed.
func (s *EdgeSyncScheduler) whenNotPaused(call func()) {
	s.callMu.Lock()
	defer s.callMu.Unlock()
	if until := s.tracker.PausedUntil(); s.now().Before(until) {
		return
	}
	call()
}

func (s *EdgeSyncScheduler) pushJob() {
	s.whenNotPaused(func() { s.asBox(s.push) })
}

func (s *EdgeSyncScheduler) push(ctx context.Context) {
	result, err := s.pushService.PushPending(ctx)
	if err != nil {
		s.recordRefusal(err)
		s.logger.ErrorContext(ctx, "Edge sync push job failed", slog.Any("error", err))
		return
	}
	if result.PushedOps > 0 {
		s.logger.InfoContext(ctx, "Edge sync push job completed",
			slog.Int("pushed_ops", result.PushedOps),
			slog.Int("batches", result.Batches),
		)
	}
}

func (s *EdgeSyncScheduler) pullJob() {
	s.whenNotPaused(func() { s.asBox(s.pull) })
}

func (s *EdgeSyncScheduler) pull(ctx context.Context) {
	result, err := s.pullService.PullChanges(ctx)
	if err != nil {
		// The pull loop always contacts the cloud, so its outcome is the edge's
		// connectivity signal: a failure flips the status endpoint to offline.
		if !s.recordRefusal(err) {
			if errors.Is(err, domainError.ErrPullApplyFailed) {
				s.tracker.RecordApplyFailure()
			} else {
				s.tracker.RecordFailure()
			}
		}
		s.logger.ErrorContext(ctx, "Edge sync pull job failed", slog.Any("error", err))
		return
	}
	s.tracker.RecordSuccess()
	if result.Products+result.Users+result.Suppliers+result.ProductResponsibilities+result.ProductIngredients+result.PlateOptionSlots > 0 {
		s.logger.InfoContext(ctx, "Edge sync pull job completed",
			slog.Int("products", result.Products),
			slog.Int("users", result.Users),
			slog.Int("suppliers", result.Suppliers),
			slog.Int("product_responsibilities", result.ProductResponsibilities),
			slog.Int("product_ingredients", result.ProductIngredients),
			slog.Int("plate_option_slots", result.PlateOptionSlots),
		)
	}
}

// recordRefusal pauses sync for the cloud's Retry-After and reports whether it recorded a reason
// the POS should show; an invalid key pauses without one, so the POS keeps showing offline.
func (s *EdgeSyncScheduler) recordRefusal(err error) bool {
	var refusal *domainError.NodeRefusalError
	if errors.As(err, &refusal) {
		until := s.now().Add(refusalPause(refusal.RetryAfter))
		s.tracker.Pause(until)
		s.logger.Warn("Edge sync paused: the cloud refused this box", slog.Any("reason", refusal.Reason), slog.Time("until", until))
	}
	switch {
	case errors.Is(err, domainError.ErrNodeRevoked):
		s.tracker.RecordBlocked(syncstatus.BlockedReplaced)
	case errors.Is(err, domainError.ErrNodeOrganizationSuspended):
		s.tracker.RecordBlocked(syncstatus.BlockedSuspended)
	default:
		return false
	}
	return true
}

func refusalPause(retryAfter time.Duration) time.Duration {
	if retryAfter <= 0 {
		return defaultRefusalPause
	}
	return min(max(retryAfter, minRefusalPause), maxRefusalPause)
}
