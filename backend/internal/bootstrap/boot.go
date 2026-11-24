package bootstrap

import (
	"fmt"

	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/httpclient"
	"laguna-escondida/backend/internal/platform/storage"
)

// Boot is a binary's loaded configuration, ready for Main to assemble once the database is up.
type Boot struct {
	Shared *config.Shared
	// Telemetry identity; empty in the cloud, which has no node or organization of its own.
	NodeID, OrganizationID string
	Assemble               func(Infra) (*App, error)
}

func CloudBoot() (Boot, error) {
	cfg, err := config.LoadCloud()
	if err != nil {
		return Boot{}, err
	}
	return Boot{
		Shared: &cfg.Shared,
		Assemble: func(infra Infra) (*App, error) {
			storageClient, err := storage.NewS3Client(cfg.Storage)
			if err != nil {
				return nil, fmt.Errorf("create storage client: %w", err)
			}
			infra.Storage = storageClient
			infra.ElectronicInvoice = httpclient.NewElectronicInvoiceClient(cfg.ElectronicInvoice, infra.HTTPClient)
			return NewCloud(cfg, infra)
		},
	}, nil
}

func EdgeBoot() (Boot, error) {
	cfg, err := config.LoadEdge()
	if err != nil {
		return Boot{}, err
	}
	return Boot{
		Shared:         &cfg.Shared,
		NodeID:         cfg.NodeID,
		OrganizationID: cfg.OrganizationID,
		Assemble: func(infra Infra) (*App, error) {
			return NewEdge(cfg, infra)
		},
	}, nil
}
