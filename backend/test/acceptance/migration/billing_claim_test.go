package migration

import (
	"context"
	"sync"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/postgres/migrations"
	"laguna-escondida/backend/internal/platform/postgres/repository"
	"laguna-escondida/backend/test/acceptance/testsupport"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Two replicas run the charge job in the same second: the billing row's lock gives the due
// organization to one of them, and the other skips it instead of waiting.
func TestBillingRepository_ConcurrentClaimsOfOneDueOrganization(t *testing.T) {
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}

	ctx := context.Background()
	base, container := resolveConn(t, ctx)
	if container != nil {
		defer func() { _ = container.Terminate(ctx) }()
	}
	const dbName = "laguna_accept_billing_claim"
	recreateDatabase(t, base, dbName)
	m, err := testsupport.NewMigrator(migrations.Cloud, base.migrateURL(dbName))
	require.NoError(t, err)
	defer func() { _, _ = m.Close() }()
	require.NoError(t, m.Up())

	db := openGorm(t, base.gormDSN(dbName))
	defer closeGorm(db)
	repo := repository.NewBillingRepository(db)
	uow := postgres.NewUnitOfWork(db)

	now := time.Date(2026, 12, 15, 12, 0, 0, 0, time.UTC)
	trialEnd := now.Add(-time.Hour)
	orgID := uuid.NewString()
	require.NoError(t, db.Exec(`INSERT INTO organizations (id, name, slug, status, plan, trial_ends_at) VALUES (?, 'Due', ?, 'trial', 'einvoicing', ?)`,
		orgID, "due-"+orgID[:8], trialEnd).Error)
	require.NoError(t, repo.SaveCard(ctx, orgID, dto.BillingCard{Provider: "wompi", PaymentSourceID: "3891", Brand: "VISA", LastFour: "4242", TermsAcceptedAt: now}))
	adminID := uuid.NewString()
	require.NoError(t, db.Exec(`INSERT INTO users (id, organization_id, username, name, email, password) VALUES (?, ?, 'owner', 'Owner', 'owner@example.com', 'x')`,
		adminID, orgID).Error)
	require.NoError(t, db.Exec(`INSERT INTO user_roles (user_id, role_id, organization_id) VALUES (?, 2, ?)`, adminID, orgID).Error)

	// Organizations the job must never see: no card, not yet due, suspended, and active by hand.
	for i, other := range []struct {
		status   string
		trialEnd *time.Time
		card     bool
	}{
		{"trial", &trialEnd, false},
		{"trial", ptrTime(now.Add(time.Hour)), true},
		{"suspended", &trialEnd, true},
		{"active", &trialEnd, true},
	} {
		id := uuid.NewString()
		require.NoError(t, db.Exec(`INSERT INTO organizations (id, name, slug, status, plan, trial_ends_at) VALUES (?, 'Other', ?, ?, 'starter', ?)`,
			id, "other-"+id[:8], other.status, other.trialEnd).Error, "organization %d", i)
		if other.card {
			require.NoError(t, repo.SaveCard(ctx, id, dto.BillingCard{Provider: "wompi", PaymentSourceID: "77", TermsAcceptedAt: now}))
		} else {
			require.NoError(t, repo.SetPeriod(ctx, id, "monthly"))
		}
	}

	claim := func(reference string, locked chan<- int, release <-chan struct{}) (int, error) {
		claimed := 0
		err := uow.Do(ctx, func(ctx context.Context) error {
			due, err := repo.LockDue(ctx, now)
			if err != nil {
				return err
			}
			if locked != nil {
				locked <- len(due)
				<-release
			}
			for _, sub := range due {
				if sub.HasOpenCharge {
					continue
				}
				charge := &dto.BillingCharge{ID: uuid.NewString(), OrganizationID: sub.OrganizationID, DueOn: sub.DueOn, Attempt: sub.Attempts + 1,
					Reference: reference, Plan: sub.Plan, Period: sub.Period, Amount: 40000, Currency: "COP", Status: dto.BillingChargeCreated,
					CreatedAt: now, UpdatedAt: now}
				if err := repo.InsertCharge(ctx, charge); err != nil {
					return err
				}
				if err := repo.IncrementAttempts(ctx, sub.OrganizationID); err != nil {
					return err
				}
				claimed++
			}
			return nil
		})
		return claimed, err
	}

	t.Run("the second claim skips the row the first holds", func(t *testing.T) {
		locked, release := make(chan int), make(chan struct{})
		var wg sync.WaitGroup
		var firstClaimed int
		var firstErr error
		wg.Add(1)
		go func() {
			defer wg.Done()
			firstClaimed, firstErr = claim("kb-first", locked, release)
		}()
		require.Equal(t, 1, <-locked, "only the due organization with a card is returned")

		secondClaimed, err := claim("kb-second", nil, nil)
		require.NoError(t, err)
		assert.Zero(t, secondClaimed, "the held row is skipped, not waited for")

		close(release)
		wg.Wait()
		require.NoError(t, firstErr)
		assert.Equal(t, 1, firstClaimed)
	})

	t.Run("after the first commits the organization has an open charge", func(t *testing.T) {
		var due []dto.DueSubscription
		require.NoError(t, uow.Do(ctx, func(ctx context.Context) error {
			var err error
			due, err = repo.LockDue(ctx, now)
			return err
		}))
		require.Len(t, due, 1)
		assert.True(t, due[0].HasOpenCharge)
		assert.Equal(t, 1, due[0].Attempts)
		assert.Equal(t, "owner@example.com", due[0].CustomerEmail)
		assert.Equal(t, "3891", due[0].PaymentSourceID)
		assert.True(t, due[0].DueOn.Equal(trialEnd))

		err := repo.InsertCharge(ctx, &dto.BillingCharge{ID: uuid.NewString(), OrganizationID: orgID, DueOn: trialEnd, Attempt: 0,
			Reference: "kb-third", Plan: "einvoicing", Period: "monthly", Amount: 40000, Currency: "COP", Status: dto.BillingChargeCreated,
			CreatedAt: now, UpdatedAt: now})
		assert.ErrorIs(t, err, domainError.ErrChargePending)
	})

	t.Run("charges are read, settled and listed", func(t *testing.T) {
		var charge *dto.BillingCharge
		require.NoError(t, uow.Do(ctx, func(ctx context.Context) error {
			found, ok, err := repo.LockChargeByReference(ctx, "kb-first")
			if err != nil || !ok {
				return err
			}
			charge = found
			return nil
		}))
		require.NotNil(t, charge)
		assert.Equal(t, dto.BillingChargeCreated, charge.Status)

		open, err := repo.OpenChargesCreatedBefore(ctx, now.Add(time.Minute))
		require.NoError(t, err)
		require.Len(t, open, 1)
		none, err := repo.OpenChargesCreatedBefore(ctx, now.Add(-time.Minute))
		require.NoError(t, err)
		assert.Empty(t, none)

		transaction := "tx_1"
		paidUntil := trialEnd.AddDate(0, 1, 0)
		charge.Status, charge.TransactionID, charge.CoversFrom, charge.CoversUntil = dto.BillingChargeApproved, &transaction, &trialEnd, &paidUntil
		require.NoError(t, repo.UpdateCharge(ctx, charge))
		require.NoError(t, repo.MarkPaid(ctx, orgID, paidUntil, false))

		byTransaction, ok, err := repo.FindChargeByTransactionID(ctx, "tx_1")
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, dto.BillingChargeApproved, byTransaction.Status)
		_, ok, err = repo.FindChargeByTransactionID(ctx, "tx_unknown")
		require.NoError(t, err)
		assert.False(t, ok)

		billing, _, err := repo.Get(ctx, orgID)
		require.NoError(t, err)
		require.NotNil(t, billing.PaidUntil)
		assert.True(t, billing.PaidUntil.Equal(paidUntil))
		assert.Zero(t, billing.Attempts)

		reader := repository.NewOrganizationAccessReader(db)
		access, err := reader.GetAccess(ctx, orgID)
		require.NoError(t, err)
		require.NotNil(t, access.PaidUntil)
		assert.True(t, access.PaidUntil.Equal(paidUntil))
		assert.Nil(t, access.CancelledAt)
		assert.True(t, access.HasCard)

		cancelledAt := now
		require.NoError(t, repo.SetCancelled(ctx, orgID, &cancelledAt))
		billing, _, _ = repo.Get(ctx, orgID)
		assert.True(t, billing.Cancelled())
		access, err = reader.GetAccess(ctx, orgID)
		require.NoError(t, err)
		require.NotNil(t, access.CancelledAt)
		assert.True(t, access.CancelledAt.Equal(cancelledAt))

		bare := uuid.NewString()
		require.NoError(t, db.Exec(`INSERT INTO organizations (id, name, slug, status, plan) VALUES (?, 'Bare', ?, 'active', 'full')`, bare, "bare-"+bare[:8]).Error)
		access, err = reader.GetAccess(ctx, bare)
		require.NoError(t, err, "an organization without a billing row reads as one without a subscription")
		assert.Nil(t, access.PaidUntil)
		assert.False(t, access.HasCard)
		require.NoError(t, repo.MarkPaid(ctx, orgID, paidUntil, true))
		billing, _, _ = repo.Get(ctx, orgID)
		assert.False(t, billing.Cancelled(), "a recovery payment resumes the subscription")

		require.NoError(t, repo.EndSubscription(ctx, orgID, now))
		billing, _, _ = repo.Get(ctx, orgID)
		assert.Nil(t, billing.PaidUntil)
		assert.True(t, billing.Cancelled())
		assert.True(t, billing.HasCard(), "the card is kept")

		charges, err := repo.ListCharges(ctx, orgID)
		require.NoError(t, err)
		require.Len(t, charges, 1)
		assert.Equal(t, "kb-first", charges[0].Reference)
	})
}

func ptrTime(t time.Time) *time.Time { return &t }
