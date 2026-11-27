package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/plans"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/tenant"
)

// BillingSetupService is what a pending admin does between signing up and the trial: read the
// setup state, choose plan and period, and leave a card, which starts the trial.
type BillingSetupService struct {
	billing  ports.BillingRepository
	orgRepo  ports.OrganizationRepository
	userRepo ports.UserRepository
	userRole ports.UserRoleRepository
	users    *UserService
	uow      ports.UnitOfWork
	// provider is nil while the cloud has no billing keys: card entry is off.
	provider ports.BillingProvider
	public   dto.BillingProviderPublic
	now      func() time.Time
}

func NewBillingSetupService(billing ports.BillingRepository, orgRepo ports.OrganizationRepository, userRepo ports.UserRepository,
	userRole ports.UserRoleRepository, users *UserService, uow ports.UnitOfWork) *BillingSetupService {
	return &BillingSetupService{billing: billing, orgRepo: orgRepo, userRepo: userRepo, userRole: userRole,
		users: users, uow: uow, now: time.Now}
}

// EnableCardEntry turns card entry on. public is what the browser needs to reach the provider.
func (s *BillingSetupService) EnableCardEntry(provider ports.BillingProvider, public dto.BillingProviderPublic) {
	s.provider = provider
	s.public = public
}

func (s *BillingSetupService) CardEntry() bool { return s.provider != nil }

func (s *BillingSetupService) State(ctx context.Context, orgID string) (*dto.BillingSetupState, error) {
	org, err := s.orgRepo.Get(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("billing setup state: %w", err)
	}
	period, err := s.period(ctx, orgID)
	if err != nil {
		return nil, err
	}
	state := &dto.BillingSetupState{OrganizationName: org.Name, Plan: org.Plan, Period: period, CardEntry: s.CardEntry()}
	setup, err := s.CardSetup(ctx)
	if err != nil {
		return nil, err
	}
	state.Wompi = setup.Wompi
	return state, nil
}

// CardSetup is what the browser needs to send a card to the provider. Acceptance tokens are
// single use, so every read asks the provider for fresh ones.
func (s *BillingSetupService) CardSetup(ctx context.Context) (*dto.BillingCardSetup, error) {
	if !s.CardEntry() {
		return &dto.BillingCardSetup{}, nil
	}
	terms, err := s.provider.Terms(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrPaymentProviderUnavailable, err)
	}
	return &dto.BillingCardSetup{CardEntry: true, Wompi: &dto.BillingSetupWompi{
		PublicKey: s.public.PublicKey, APIURL: s.public.APIURL,
		TermsURL: terms.TermsURL, PersonalDataURL: terms.PersonalDataURL,
		AcceptanceToken: terms.AcceptanceToken, PersonalDataAcceptanceToken: terms.PersonalAuthToken,
	}}, nil
}

func (s *BillingSetupService) period(ctx context.Context, orgID string) (string, error) {
	billing, found, err := s.billing.Get(ctx, orgID)
	if err != nil {
		return "", fmt.Errorf("billing setup: read billing: %w", err)
	}
	if !found {
		return string(plans.Monthly), nil
	}
	return billing.Period, nil
}

// Select stores the plan and period; the amount it answers comes from the price table, never the request.
func (s *BillingSetupService) Select(ctx context.Context, orgID string, req *dto.BillingSelectionRequest) (*dto.BillingSelection, error) {
	return s.selectWhen(ctx, orgID, req, func(_ context.Context, org *dto.Organization) error {
		if org.Status != dto.OrganizationStatusPending {
			return domainError.ErrOrganizationNotPending
		}
		return nil
	})
}

// selectWhen stores the selection only for an organization allowed passes, checked inside the transaction.
func (s *BillingSetupService) selectWhen(ctx context.Context, orgID string, req *dto.BillingSelectionRequest,
	allowed func(ctx context.Context, org *dto.Organization) error) (*dto.BillingSelection, error) {
	if !plans.Valid(req.Plan) {
		return nil, &domainError.BillingValidationError{Field: "plan", Reason: "must be starter, einvoicing or full"}
	}
	if !plans.ValidPeriod(req.Period) {
		return nil, &domainError.BillingValidationError{Field: "period", Reason: "must be monthly or annual"}
	}
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		org, err := s.orgRepo.Get(ctx, orgID)
		if err != nil {
			return err
		}
		if err := allowed(ctx, org); err != nil {
			return err
		}
		if org.Plan != req.Plan {
			if _, err := s.orgRepo.Update(ctx, orgID, &dto.UpdateOrganizationRequest{Plan: &req.Plan}); err != nil {
				return err
			}
		}
		return s.billing.SetPeriod(ctx, orgID, req.Period)
	})
	if err != nil {
		return nil, fmt.Errorf("billing selection: %w", err)
	}
	return &dto.BillingSelection{Plan: req.Plan, Period: req.Period,
		Amount: plans.Price(plans.Plan(req.Plan), plans.Period(req.Period)), Currency: plans.Currency}, nil
}

// SaveCard creates the payment source first and then, in one transaction, stores the card and
// starts the trial. If the transaction fails the source stays at the provider unreferenced: it
// costs and charges nothing, and the person retries with a new card token.
func (s *BillingSetupService) SaveCard(ctx context.Context, orgID, userID string, req *dto.SaveCardRequest) (*dto.SignInResponse, error) {
	if err := s.validateCard(req); err != nil {
		return nil, err
	}
	ctx = tenant.WithOrg(ctx, orgID)
	org, err := s.orgRepo.Get(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("save card: %w", err)
	}
	if org.Status != dto.OrganizationStatusPending {
		return nil, domainError.ErrOrganizationNotPending
	}
	card, admin, err := s.newCard(ctx, orgID, userID, req)
	if err != nil {
		return nil, fmt.Errorf("save card: %w", err)
	}

	err = s.uow.Do(ctx, func(ctx context.Context) error {
		if saveErr := s.billing.SaveCard(ctx, orgID, card); saveErr != nil {
			return saveErr
		}
		// The same conditional update the operator's start-trial uses: it changes nothing unless still pending.
		_, startErr := s.orgRepo.StartTrial(ctx, orgID, card.TermsAcceptedAt.Add(plans.TrialLength))
		return startErr
	})
	if err != nil {
		return nil, fmt.Errorf("save card: %w", err)
	}

	roles, err := s.userRole.FindRolesByUserID(ctx, admin.ID)
	if err != nil {
		return nil, fmt.Errorf("save card: read roles: %w", err)
	}
	return s.users.issueSignIn(admin, orgID, roles)
}

// newCard turns a card token into a payment source at the provider, for the user who leaves it.
// Nothing is stored: the caller saves the card in its own transaction.
func (s *BillingSetupService) newCard(ctx context.Context, orgID, userID string, req *dto.SaveCardRequest) (dto.BillingCard, *dto.User, error) {
	if err := s.validateCard(req); err != nil {
		return dto.BillingCard{}, nil, err
	}
	admin, err := s.cardHolder(tenant.WithOrg(ctx, orgID), userID)
	if err != nil {
		return dto.BillingCard{}, nil, err
	}
	source, err := s.provider.SavePaymentSource(ctx, dto.PaymentSourceRequest{
		CardToken: req.CardToken, CustomerEmail: *admin.Email,
		AcceptanceToken: req.AcceptanceToken, PersonalAuthToken: req.PersonalDataAcceptanceToken,
	})
	if err != nil {
		return dto.BillingCard{}, nil, err
	}
	return dto.BillingCard{Provider: dto.BillingProviderWompi, PaymentSourceID: source.ID,
		Brand: source.Brand, LastFour: source.LastFour, TermsAcceptedAt: s.now().UTC()}, admin, nil
}

func (s *BillingSetupService) validateCard(req *dto.SaveCardRequest) error {
	if !s.CardEntry() {
		return domainError.ErrCardEntryOff
	}
	for _, f := range []struct{ name, value string }{
		{"card_token", req.CardToken},
		{"acceptance_token", req.AcceptanceToken},
		{"personal_data_acceptance_token", req.PersonalDataAcceptanceToken},
	} {
		if strings.TrimSpace(f.value) == "" {
			return &domainError.BillingValidationError{Field: f.name, Reason: "is required"}
		}
	}
	return nil
}

// cardHolder is the active user, with an email, in whose name a card is saved or charged.
func (s *BillingSetupService) cardHolder(ctx context.Context, userID string) (*dto.User, error) {
	admin, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}
	if !admin.Active || admin.Email == nil || *admin.Email == "" {
		return nil, errors.New("the user cannot hold a card: inactive or without an email")
	}
	return admin, nil
}
