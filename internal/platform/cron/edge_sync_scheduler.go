package cron

import (
	"context"

	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/syncstatus"

	"github.com/go-co-op/gocron/v2"
	"go.uber.org/zap"
)

// EdgeSyncScheduler runs the edge sync loops on timers: the push loop drains this node's
// unsynced outbox to the cloud, and the pull loop applies the cloud's reference changes.
// It owns its own gocron scheduler so the edge sync concern is started and stopped
// independently of the cloud cron jobs.
type EdgeSyncScheduler struct {
	scheduler   gocron.Scheduler
	pushService *service.SyncPushService
	pullService *service.SyncPullService
	tracker     *syncstatus.Tracker
	pushCron    string
	pullCron    string
	logger      *zap.Logger
}

func NewEdgeSyncScheduler(
	pushService *service.SyncPushService,
	pullService *service.SyncPullService,
	tracker *syncstatus.Tracker,
	pushCron string,
	pullCron string,
	logger *zap.Logger,
) (*EdgeSyncScheduler, error) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}

	return &EdgeSyncScheduler{
		scheduler:   scheduler,
		pushService: pushService,
		pullService: pullService,
		tracker:     tracker,
		pushCron:    pushCron,
		pullCron:    pullCron,
		logger:      logger,
	}, nil
}

func (s *EdgeSyncScheduler) Start() error {
	if _, err := s.scheduler.NewJob(
		gocron.CronJob(s.pushCron, false),
		gocron.NewTask(s.pushJob),
	); err != nil {
		s.logger.Error("Failed to register sync push cron job", zap.Error(err))
		return err
	}

	if _, err := s.scheduler.NewJob(
		gocron.CronJob(s.pullCron, false),
		gocron.NewTask(s.pullJob),
	); err != nil {
		s.logger.Error("Failed to register sync pull cron job", zap.Error(err))
		return err
	}

	s.scheduler.Start()
	s.logger.Info("Edge sync scheduler started",
		zap.String("push_cron", s.pushCron),
		zap.String("pull_cron", s.pullCron),
	)
	return nil
}

func (s *EdgeSyncScheduler) Stop() error {
	s.logger.Info("Stopping edge sync scheduler...")
	return s.scheduler.Shutdown()
}

func (s *EdgeSyncScheduler) pushJob() {
	ctx := context.Background()
	result, err := s.pushService.PushPending(ctx)
	if err != nil {
		s.logger.Error("Edge sync push job failed", zap.Error(err))
		return
	}
	if result.PushedOps > 0 {
		s.logger.Info("Edge sync push job completed",
			zap.Int("pushed_ops", result.PushedOps),
			zap.Int("batches", result.Batches),
		)
	}
}

func (s *EdgeSyncScheduler) pullJob() {
	ctx := context.Background()
	result, err := s.pullService.PullChanges(ctx)
	if err != nil {
		// The pull loop always contacts the cloud, so its outcome is the edge's
		// connectivity signal: a failure flips the status endpoint to offline.
		s.tracker.RecordFailure()
		s.logger.Error("Edge sync pull job failed", zap.Error(err))
		return
	}
	s.tracker.RecordSuccess()
	if result.Products+result.Users+result.Suppliers > 0 {
		s.logger.Info("Edge sync pull job completed",
			zap.Int("products", result.Products),
			zap.Int("users", result.Users),
			zap.Int("suppliers", result.Suppliers),
		)
	}
}
