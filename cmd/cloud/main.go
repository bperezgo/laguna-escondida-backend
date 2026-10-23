package main

import (
	"laguna-escondida/backend/internal/bootstrap"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/httpclient"
)

func main() {
	bootstrap.Main(config.ModeCloud, func(cfg *config.Config, infra bootstrap.Infra) (*bootstrap.App, error) {
		infra.ElectronicInvoice = httpclient.NewElectronicInvoiceClient(cfg, infra.HTTPClient)
		return bootstrap.NewCloud(cfg, infra)
	})
}
