package cron

import (
	"context"
	"log/slog"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/go-co-op/gocron/v2"
)

// marketingDeliveryInterval is how often due events are looked for; the first retry waits 10s.
const marketingDeliveryInterval = 5 * time.Second

// CRMJobs are the cloud's reservation jobs; Delivery is nil while marketing events are off.
type CRMJobs struct {
	Reservations *service.ReservationService
	Feedback     *service.FeedbackService
	Delivery     *service.MarketingEventDeliveryService
}

// CRMScheduler marks visits, asks for feedback and sends marketing events, once per organization.
type CRMScheduler struct {
	scheduler     gocron.Scheduler
	runner        *service.OrganizationRunner
	organizations ports.OrganizationRepository
	secrets       ports.MarketingEventSecrets
	jobs          CRMJobs
	visitedCron   string
	feedbackCron  string
	logger        *slog.Logger
}

func NewCRMScheduler(
	runner *service.OrganizationRunner,
	organizations ports.OrganizationRepository,
	secrets ports.MarketingEventSecrets,
	jobs CRMJobs,
	visitedCron, feedbackCron string,
	logger *slog.Logger,
) (*CRMScheduler, error) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}
	return &CRMScheduler{
		scheduler: scheduler, runner: runner, organizations: organizations, secrets: secrets, jobs: jobs,
		visitedCron: visitedCron, feedbackCron: feedbackCron, logger: logger,
	}, nil
}

func (s *CRMScheduler) Start() error {
	if err := s.registerJobs(); err != nil {
		return err
	}
	s.warnOrganizationsWithoutSecret(context.Background())
	s.scheduler.Start()
	s.logger.Info("CRM scheduler started")
	return nil
}

// Stop waits for running jobs, so a send in flight settles its row before the process exits.
func (s *CRMScheduler) Stop() error {
	s.logger.Info("Stopping CRM scheduler...")
	return s.scheduler.Shutdown()
}

func (s *CRMScheduler) registerJobs() error {
	if _, err := s.scheduler.NewJob(gocron.CronJob(s.visitedCron, false), gocron.NewTask(s.markVisitedJob),
		gocron.WithSingletonMode(gocron.LimitModeReschedule)); err != nil {
		s.logger.Error("Failed to register markReservationsVisited cron job", slog.Any("error", err))
		return err
	}
	s.logger.Info("Registered cron job: markReservationsVisited", slog.String("cron", s.visitedCron))

	if _, err := s.scheduler.NewJob(gocron.CronJob(s.feedbackCron, false), gocron.NewTask(s.requestFeedbackJob),
		gocron.WithSingletonMode(gocron.LimitModeReschedule)); err != nil {
		s.logger.Error("Failed to register requestFeedback cron job", slog.Any("error", err))
		return err
	}
	s.logger.Info("Registered cron job: requestFeedback", slog.String("cron", s.feedbackCron))

	if s.jobs.Delivery == nil {
		s.logger.Info("Marketing events disabled: set MARKETING_APP_EVENTS_URL to send them to marketing-app")
		return nil
	}
	if _, err := s.scheduler.NewJob(gocron.DurationJob(marketingDeliveryInterval), gocron.NewTask(s.deliverMarketingEventsJob),
		gocron.WithSingletonMode(gocron.LimitModeReschedule)); err != nil {
		s.logger.Error("Failed to register deliverMarketingEvents job", slog.Any("error", err))
		return err
	}
	s.logger.Info("Registered job: deliverMarketingEvents", slog.Duration("every", marketingDeliveryInterval))
	return nil
}

// warnOrganizationsWithoutSecret names each active organization whose events would be dropped.
func (s *CRMScheduler) warnOrganizationsWithoutSecret(ctx context.Context) {
	if s.jobs.Delivery == nil {
		return
	}
	orgs, err := s.organizations.List(ctx)
	if err != nil {
		s.logger.ErrorContext(ctx, "Could not list organizations to check marketing event secrets", slog.Any("error", err))
		return
	}
	for _, org := range orgs {
		if org.Status != dto.OrganizationStatusActive {
			continue
		}
		if _, ok := s.secrets.SecretFor(org.Slug); !ok {
			s.logger.WarnContext(ctx, "Organization has no marketing event signing secret; its agent orders and reservations record no events",
				slog.String("organization_id", org.ID), slog.String("slug", org.Slug))
		}
	}
}

func (s *CRMScheduler) markVisitedJob() {
	s.forEachOrganization("markReservationsVisited", func(ctx context.Context) error {
		marked, err := s.jobs.Reservations.MarkVisited(ctx)
		if marked > 0 {
			orgID, _ := tenant.OrgFrom(ctx)
			s.logger.InfoContext(ctx, "Reservations marked visited", slog.String("organization_id", orgID), slog.Int("count", marked))
		}
		return err
	})
}

func (s *CRMScheduler) requestFeedbackJob() {
	s.forEachOrganization("requestFeedback", func(ctx context.Context) error {
		requested, err := s.jobs.Feedback.RequestDue(ctx)
		if requested > 0 {
			orgID, _ := tenant.OrgFrom(ctx)
			s.logger.InfoContext(ctx, "Feedback requested", slog.String("organization_id", orgID), slog.Int("count", requested))
		}
		return err
	})
}

func (s *CRMScheduler) deliverMarketingEventsJob() {
	s.forEachOrganization("deliverMarketingEvents", func(ctx context.Context) error {
		_, err := s.jobs.Delivery.DeliverDue(ctx)
		return err
	})
}

func (s *CRMScheduler) forEachOrganization(job string, fn func(ctx context.Context) error) {
	ctx := context.Background()
	failures, err := s.runner.ForEach(ctx, service.AllActiveOrganizations, fn)
	if err != nil {
		s.logger.ErrorContext(ctx, "Cron job "+job+" could not list organizations", slog.Any("error", err))
		return
	}
	for _, failure := range failures {
		s.logger.ErrorContext(ctx, "Cron job "+job+" failed",
			slog.String("organization_id", failure.OrganizationID), slog.Any("error", failure.Err))
	}
}
