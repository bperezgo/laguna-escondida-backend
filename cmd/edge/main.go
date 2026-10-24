package main

import (
	"laguna-escondida/backend/internal/bootstrap"
	"laguna-escondida/backend/internal/platform/config"
)

func main() {
	bootstrap.Main(config.ModeEdge, bootstrap.EdgeBoot)
}
