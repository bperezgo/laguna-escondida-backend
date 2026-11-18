package bootstrap

import (
	"fmt"
	"log/slog"

	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/httpclient"
	"laguna-escondida/backend/internal/platform/storage"
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
	Storage    ports.StorageClient
	// ElectronicInvoice is cloud-only; the edge leaves it nil.
	ElectronicInvoice ports.ElectronicInvoiceClient
	EventBus          *eventbus.GoChannelEventBus
}

// NewInfra builds the clients shared by both modes.
func NewInfra(cfg *config.Shared, db *gorm.DB, logger *slog.Logger) (Infra, error) {
	storageClient, err := storage.NewS3Client(cfg.Storage)
	if err != nil {
		return Infra{}, fmt.Errorf("create storage client: %w", err)
	}
	return Infra{
		DB:         db,
		Logger:     logger,
		HTTPClient: httpclient.NewClient(logger),
		Storage:    storageClient,
		EventBus:   eventbus.NewGoChannelEventBus(watermill.NewSlogLogger(logger)),
	}, nil
}
