package bootstrap

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/platform/config"

	"github.com/stretchr/testify/assert"
)

func TestLogPaymentLinks_ReportsEachActiveOrganizationAndLeftoverEnv(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	orgs := mocks.NewMockOrganizationRepository(t)
	orgs.EXPECT().List(context.Background()).Return([]*dto.Organization{
		{ID: "org-laguna", Slug: "laguna-escondida", Status: dto.OrganizationStatusActive},
		{ID: "org-x", Slug: "restaurante-x", Status: dto.OrganizationStatusActive},
		{ID: "org-gone", Slug: "suspendido", Status: dto.OrganizationStatusSuspended},
	}, nil).Once()
	links := mocks.NewMockPaymentLinkConfig(t)
	links.EXPECT().For(context.Background(), "org-laguna").Return(dto.WompiCredentials{PrivateKey: "prv_test_value"}, true, nil).Once()
	links.EXPECT().For(context.Background(), "org-x").Return(dto.WompiCredentials{}, false, nil).Once()

	logPaymentLinks(context.Background(), logger, orgs, links, config.PaymentLinks{
		APIURL: "https://sandbox.wompi.co/v1", IgnoredEnv: []string{"WOMPI_PRIVATE_KEY_LAGUNA_ESCONDIDA"},
	})

	out := logs.String()
	assert.Contains(t, out, "variable=WOMPI_PRIVATE_KEY_LAGUNA_ESCONDIDA")
	assert.Contains(t, out, `msg="Payment links configured" organization=laguna-escondida`)
	assert.Contains(t, out, "Payment links not configured")
	assert.Contains(t, out, "organization=restaurante-x")
	assert.NotContains(t, out, "suspendido")
	assert.NotContains(t, out, "prv_test_value")
}

func TestLogIgnoredMarketingEventsEnv_NamesTheVariableNotItsValue(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	logIgnoredMarketingEventsEnv(logger, config.MarketingEvents{
		URL: "https://marketing.example.com", IgnoredEnv: []string{"MARKETING_APP_EVENTS_SECRET_LAGUNA_ESCONDIDA"},
	})

	out := logs.String()
	assert.Contains(t, out, "level=WARN")
	assert.Contains(t, out, "Ignored: marketing event signing secrets")
	assert.Contains(t, out, "variable=MARKETING_APP_EVENTS_SECRET_LAGUNA_ESCONDIDA")
}
