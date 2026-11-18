package service

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/google/uuid"
)

const (
	defaultOrganizationTimezone = "America/Bogota"
	defaultOrganizationCountry  = "CO"
	defaultOrganizationLocale   = "es-CO"
	defaultOrganizationCurrency = "COP"

	maxOrganizationTextLength   = 255
	maxOrganizationTicketLength = 1000
)

var (
	organizationSlugPattern   = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{1,38}[a-z0-9])$`)
	organizationCountryRegex  = regexp.MustCompile(`^[A-Z]{2}$`)
	organizationLocaleRegex   = regexp.MustCompile(`^[a-z]{2}-[A-Z]{2}$`)
	organizationCurrencyRegex = regexp.MustCompile(`^[A-Z]{3}$`)
	organizationColorRegex    = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	einvoicingPrefixRegex     = regexp.MustCompile(`^[A-Z0-9]{1,10}$`)
)

type OrganizationService struct {
	orgRepo        ports.OrganizationRepository
	einvoicingRepo ports.EInvoicingSettingsRepository
	uow            ports.UnitOfWork
}

func NewOrganizationService(
	orgRepo ports.OrganizationRepository,
	einvoicingRepo ports.EInvoicingSettingsRepository,
	uow ports.UnitOfWork,
) *OrganizationService {
	return &OrganizationService{orgRepo: orgRepo, einvoicingRepo: einvoicingRepo, uow: uow}
}

func (s *OrganizationService) Create(ctx context.Context, req *dto.CreateOrganizationRequest) (*dto.Organization, error) {
	if err := validateCreateOrganization(req); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	org := &dto.Organization{
		ID:                uuid.Must(uuid.NewV7()).String(),
		Name:              strings.TrimSpace(req.Name),
		Slug:              req.Slug,
		Status:            orDefault(req.Status, dto.OrganizationStatusActive),
		Timezone:          orDefault(req.Timezone, defaultOrganizationTimezone),
		Country:           orDefault(req.Country, defaultOrganizationCountry),
		Locale:            orDefault(req.Locale, defaultOrganizationLocale),
		Currency:          orDefault(req.Currency, defaultOrganizationCurrency),
		LegalName:         nilIfEmpty(req.LegalName),
		NIT:               nilIfEmpty(req.NIT),
		Address:           nilIfEmpty(req.Address),
		Phone:             nilIfEmpty(req.Phone),
		TicketFooter:      nilIfEmpty(req.TicketFooter),
		TicketLegalNotice: nilIfEmpty(req.TicketLegalNotice),
		DisplayName:       nilIfEmpty(req.DisplayName),
		LogoURL:           nilIfEmpty(req.LogoURL),
		PrimaryColor:      nilIfEmpty(req.PrimaryColor),
		SecondaryColor:    nilIfEmpty(req.SecondaryColor),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if req.EInvoicing != nil {
		org.EInvoicing = &dto.EInvoicingSettings{
			OrganizationID:        org.ID,
			InvoicePrefix:         req.EInvoicing.InvoicePrefix,
			SupportDocumentPrefix: req.EInvoicing.SupportDocumentPrefix,
		}
	}

	err := s.uow.Do(ctx, func(ctx context.Context) error {
		if err := s.orgRepo.Create(ctx, org); err != nil {
			return err
		}
		if org.EInvoicing != nil {
			return s.einvoicingRepo.Upsert(ctx, org.EInvoicing)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create organization: %w", err)
	}
	return org, nil
}

func (s *OrganizationService) List(ctx context.Context) ([]*dto.Organization, error) {
	orgs, err := s.orgRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list organizations: %w", err)
	}
	return orgs, nil
}

func (s *OrganizationService) Get(ctx context.Context, id string) (*dto.Organization, error) {
	org, err := s.orgRepo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get organization: %w", err)
	}
	return org, nil
}

func (s *OrganizationService) GetCurrent(ctx context.Context, id string) (*dto.CurrentOrganization, error) {
	org, err := s.orgRepo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get current organization: %w", err)
	}
	return &dto.CurrentOrganization{
		ID: org.ID, Slug: org.Slug, Name: org.Name,
		Timezone: org.Timezone, Country: org.Country, Locale: org.Locale, Currency: org.Currency,
		LegalName: org.LegalName, NIT: org.NIT, Address: org.Address, Phone: org.Phone,
		TicketFooter: org.TicketFooter, TicketLegalNotice: org.TicketLegalNotice,
		DisplayName: org.DisplayName, LogoURL: org.LogoURL,
		PrimaryColor: org.PrimaryColor, SecondaryColor: org.SecondaryColor,
	}, nil
}

func (s *OrganizationService) Update(ctx context.Context, id string, req *dto.UpdateOrganizationRequest) (*dto.Organization, error) {
	if err := validateUpdateOrganization(req); err != nil {
		return nil, err
	}
	normalized := *req
	if normalized.Name != nil {
		name := strings.TrimSpace(*normalized.Name)
		normalized.Name = &name
	}
	org, err := s.orgRepo.Update(ctx, id, &normalized)
	if err != nil {
		return nil, fmt.Errorf("update organization: %w", err)
	}
	return org, nil
}

// SetEInvoicing replaces the prefixes in place; counters are per (organization, prefix), so a new prefix starts at 1.
func (s *OrganizationService) SetEInvoicing(ctx context.Context, id string, req *dto.SetEInvoicingRequest) (*dto.EInvoicingSettings, error) {
	if err := validateEInvoicingPrefixes(req); err != nil {
		return nil, err
	}
	if _, err := s.orgRepo.Get(ctx, id); err != nil {
		return nil, fmt.Errorf("set e-invoicing: %w", err)
	}
	settings := &dto.EInvoicingSettings{
		OrganizationID:        id,
		InvoicePrefix:         req.InvoicePrefix,
		SupportDocumentPrefix: req.SupportDocumentPrefix,
	}
	if err := s.einvoicingRepo.Upsert(ctx, settings); err != nil {
		return nil, fmt.Errorf("set e-invoicing: %w", err)
	}
	return settings, nil
}

func validateCreateOrganization(req *dto.CreateOrganizationRequest) error {
	if err := validateOrganizationName(req.Name); err != nil {
		return err
	}
	if !organizationSlugPattern.MatchString(req.Slug) {
		return invalidOrganizationField("slug", "must be 3-40 lowercase letters, digits or hyphens, without a leading or trailing hyphen")
	}
	required := []struct {
		field, value string
		check        func(string, string) error
	}{
		{"status", req.Status, validateOrganizationStatus},
		{"timezone", req.Timezone, validateOrganizationTimezone},
		{"country", req.Country, validateOrganizationCountry},
		{"locale", req.Locale, validateOrganizationLocale},
		{"currency", req.Currency, validateOrganizationCurrency},
	}
	for _, r := range required {
		if r.value == "" {
			continue
		}
		if err := r.check(r.field, r.value); err != nil {
			return err
		}
	}
	optionalFields := []struct {
		field, value string
	}{
		{"legal_name", req.LegalName}, {"nit", req.NIT}, {"address", req.Address}, {"phone", req.Phone},
		{"ticket_footer", req.TicketFooter}, {"ticket_legal_notice", req.TicketLegalNotice},
		{"display_name", req.DisplayName}, {"logo_url", req.LogoURL},
		{"primary_color", req.PrimaryColor}, {"secondary_color", req.SecondaryColor},
	}
	for _, o := range optionalFields {
		if o.value == "" {
			continue
		}
		if err := validateOptionalOrganizationField(o.field, o.value); err != nil {
			return err
		}
	}
	if req.EInvoicing != nil {
		return validateEInvoicingPrefixes(req.EInvoicing)
	}
	return nil
}

func validateUpdateOrganization(req *dto.UpdateOrganizationRequest) error {
	if req.Name != nil {
		if err := validateOrganizationName(*req.Name); err != nil {
			return err
		}
	}
	required := []struct {
		field string
		value *string
		check func(string, string) error
	}{
		{"status", req.Status, validateOrganizationStatus},
		{"timezone", req.Timezone, validateOrganizationTimezone},
		{"country", req.Country, validateOrganizationCountry},
		{"locale", req.Locale, validateOrganizationLocale},
		{"currency", req.Currency, validateOrganizationCurrency},
	}
	for _, r := range required {
		if r.value == nil {
			continue
		}
		if *r.value == "" {
			return invalidOrganizationField(r.field, "is required and cannot be cleared")
		}
		if err := r.check(r.field, *r.value); err != nil {
			return err
		}
	}
	optionalFields := []struct {
		field string
		value *string
	}{
		{"legal_name", req.LegalName}, {"nit", req.NIT}, {"address", req.Address}, {"phone", req.Phone},
		{"ticket_footer", req.TicketFooter}, {"ticket_legal_notice", req.TicketLegalNotice},
		{"display_name", req.DisplayName}, {"logo_url", req.LogoURL},
		{"primary_color", req.PrimaryColor}, {"secondary_color", req.SecondaryColor},
	}
	for _, o := range optionalFields {
		if o.value == nil || *o.value == "" {
			continue
		}
		if err := validateOptionalOrganizationField(o.field, *o.value); err != nil {
			return err
		}
	}
	return nil
}

func validateOrganizationName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return invalidOrganizationField("name", "is required")
	}
	if utf8.RuneCountInString(trimmed) > maxOrganizationTextLength {
		return invalidOrganizationField("name", fmt.Sprintf("must be at most %d characters", maxOrganizationTextLength))
	}
	return nil
}

func validateOrganizationStatus(field, value string) error {
	if value != dto.OrganizationStatusActive && value != dto.OrganizationStatusSuspended {
		return invalidOrganizationField(field, "must be active or suspended")
	}
	return nil
}

func validateOrganizationTimezone(field, value string) error {
	// LoadLocation accepts "Local", which is the server's zone, not an IANA name.
	if value == "Local" {
		return invalidOrganizationField(field, "must be an IANA time zone name")
	}
	if _, err := time.LoadLocation(value); err != nil {
		return invalidOrganizationField(field, "must be an IANA time zone name")
	}
	return nil
}

func validateOrganizationCountry(field, value string) error {
	if !organizationCountryRegex.MatchString(value) {
		return invalidOrganizationField(field, "must be two uppercase letters (ISO 3166-1 alpha-2)")
	}
	return nil
}

func validateOrganizationLocale(field, value string) error {
	if !organizationLocaleRegex.MatchString(value) {
		return invalidOrganizationField(field, "must look like es-CO")
	}
	return nil
}

func validateOrganizationCurrency(field, value string) error {
	if !organizationCurrencyRegex.MatchString(value) {
		return invalidOrganizationField(field, "must be three uppercase letters (ISO 4217)")
	}
	return nil
}

func validateOptionalOrganizationField(field, value string) error {
	maxLength := maxOrganizationTextLength
	if field == "ticket_footer" || field == "ticket_legal_notice" {
		maxLength = maxOrganizationTicketLength
	}
	if utf8.RuneCountInString(value) > maxLength {
		return invalidOrganizationField(field, fmt.Sprintf("must be at most %d characters", maxLength))
	}
	switch field {
	case "logo_url":
		parsed, err := url.Parse(value)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return invalidOrganizationField(field, "must be an absolute https URL")
		}
	case "primary_color", "secondary_color":
		if !organizationColorRegex.MatchString(value) {
			return invalidOrganizationField(field, "must be a #RRGGBB hex color")
		}
	}
	return nil
}

func validateEInvoicingPrefixes(req *dto.SetEInvoicingRequest) error {
	if !einvoicingPrefixRegex.MatchString(req.InvoicePrefix) {
		return invalidOrganizationField("invoice_prefix", "must be 1-10 uppercase letters or digits")
	}
	if !einvoicingPrefixRegex.MatchString(req.SupportDocumentPrefix) {
		return invalidOrganizationField("support_document_prefix", "must be 1-10 uppercase letters or digits")
	}
	// Both documents count in invoice_sequences by (organization, prefix); one prefix would share a counter.
	if req.InvoicePrefix == req.SupportDocumentPrefix {
		return invalidOrganizationField("support_document_prefix", "must differ from invoice_prefix")
	}
	return nil
}

func invalidOrganizationField(field, reason string) error {
	return &domainError.OrganizationValidationError{Field: field, Reason: reason}
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func nilIfEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
