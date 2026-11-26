package bootstrap

import (
	"log/slog"

	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/httpclient"
	"laguna-escondida/backend/pkg/eventbus"

	"github.com/ThreeDotsLabs/watermill"
	"gorm.io/gorm"
)

// Infra is what an assembly needs from the outside world. Tests build their own to drive the
// real router against a test database and stub external clients.
type Infra struct {
	DB         *gorm.DB
	Logger     *slog.Logger
	HTTPClient *httpclient.Client
	// Storage is cloud-only; the cloud boot fills it and the edge leaves it nil.
	Storage ports.StorageClient
	// ElectronicInvoice is cloud-only; the edge leaves it nil.
	ElectronicInvoice ports.ElectronicInvoiceClient
	EventBus          *eventbus.GoChannelEventBus
	// GoogleVerifier stands in for Google's in tests; nil builds the real one from GOOGLE_CLIENT_ID.
	GoogleVerifier ports.GoogleIdentityVerifier
}

// NewInfra builds the clients shared by both modes.
func NewInfra(cfg *config.Shared, db *gorm.DB, logger *slog.Logger) (Infra, error) {
	return Infra{
		DB:         db,
		Logger:     logger,
		HTTPClient: httpclient.NewClient(logger),
		EventBus:   eventbus.NewGoChannelEventBus(watermill.NewSlogLogger(logger)),
	}, nil
}
