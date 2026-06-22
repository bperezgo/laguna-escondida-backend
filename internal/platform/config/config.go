package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
)

// Mode determines how the backend runs: as the cloud aggregate or as a
// restaurant edge node. The wiring in cmd/main.go branches on this value.
type Mode string

const (
	ModeCloud Mode = "cloud"
	ModeEdge  Mode = "edge"
)

type Config struct {
	AppMode                   Mode
	NodeID                    string
	NodeSyncKey               string
	CloudSyncURL              string
	CloudNodeID               string
	SyncPushCron              string
	SyncPullCron              string
	ElectronicInvoiceURL      string
	ElectronicInvoiceUser     string
	ElectronicInvoicePassword string
	ElectronicInvoicePrefix   string
	SupportDocumentPrefix     string
	AdminAPIKey               string
	JWTSecret                 string
	SpacesRegion              string
	SpacesEndpoint            string
	SpacesKey                 string
	SpacesSecret              string
	SpacesBucket              string
	OrganizationID            string
	InvoiceURLCron            string
	SupportDocumentURLCron    string
}

func NewConfig() (*Config, error) {
	appMode := Mode(os.Getenv("APP_MODE"))
	if appMode == "" {
		appMode = ModeCloud
	}
	if appMode != ModeCloud && appMode != ModeEdge {
		return nil, fmt.Errorf("invalid APP_MODE %q: must be %q or %q", appMode, ModeCloud, ModeEdge)
	}

	url := os.Getenv("ELECTRONIC_INVOICE_URL")
	if url == "" {
		return nil, errors.New("ELECTRONIC_INVOICE_URL is not set")
	}
	user := os.Getenv("ELECTRONIC_INVOICE_USER")
	if user == "" {
		return nil, errors.New("ELECTRONIC_INVOICE_USER is not set")
	}
	password := os.Getenv("ELECTRONIC_INVOICE_PASSWORD")
	if password == "" {
		return nil, errors.New("ELECTRONIC_INVOICE_PASSWORD is not set")
	}
	adminAPIKey := os.Getenv("ADMIN_API_KEY")
	if adminAPIKey == "" {
		return nil, errors.New("ADMIN_API_KEY is not set")
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, errors.New("JWT_SECRET is not set")
	}

	invoicePrefix := os.Getenv("ELECTRONIC_INVOICE_PREFIX")
	if invoicePrefix == "" {
		invoicePrefix = "SETP"
	}

	supportDocumentPrefix := os.Getenv("SUPPORT_DOCUMENT_PREFIX")
	if supportDocumentPrefix == "" {
		supportDocumentPrefix = "SETP"
	}

	spacesRegion := os.Getenv("SPACES_REGION")
	if spacesRegion == "" {
		return nil, errors.New("SPACES_REGION is not set")
	}

	// SPACES_ENDPOINT is optional: when set (e.g. http://localhost:9000 for a
	// local MinIO), it overrides the default DigitalOcean Spaces endpoint.
	spacesEndpoint := os.Getenv("SPACES_ENDPOINT")
	spacesKey := os.Getenv("SPACES_KEY")
	if spacesKey == "" {
		return nil, errors.New("SPACES_KEY is not set")
	}
	spacesSecret := os.Getenv("SPACES_SECRET")
	if spacesSecret == "" {
		return nil, errors.New("SPACES_SECRET is not set")
	}
	spacesBucket := os.Getenv("SPACES_BUCKET")
	if spacesBucket == "" {
		return nil, errors.New("SPACES_BUCKET is not set")
	}
	// TODO Review the best way to handle the organization id (securely) when the goal was to roll out this for different organizations
	organizationID := os.Getenv("ORGANIZATION_ID")
	if organizationID == "" {
		return nil, errors.New("ORGANIZATION_ID is not set")
	}

	// NodeID is this install's sync identity (origin_node_id on outbox rows). When
	// unset, derive a stable id from the organization + run mode so dev boots without
	// extra config and the id survives restarts — changing it would break per-origin
	// sync sequencing. Multi-node deployments must set NODE_ID so each install differs.
	nodeID := os.Getenv("NODE_ID")
	if nodeID == "" {
		nodeID = uuid.NewSHA1(uuid.NameSpaceURL, []byte("laguna-escondida/node/"+organizationID+"/"+string(appMode))).String()
	}

	// NodeSyncKey authenticates edge nodes calling the cloud sync endpoints
	// (NodeAuthMiddleware). Optional here so non-sync deployments boot; when unset the
	// middleware fails closed and rejects every request. Cloud installs must set it.
	nodeSyncKey := os.Getenv("NODE_SYNC_KEY")

	// CloudSyncURL is the base URL of the cloud the edge push loop targets (e.g.
	// http://localhost:8081). Optional: when unset (or NODE_SYNC_KEY is unset), the
	// edge push loop stays disabled rather than failing to boot. Cloud installs ignore it.
	cloudSyncURL := os.Getenv("CLOUD_SYNC_URL")

	// CloudNodeID is the peer identity the edge advances last_pushed_seq against. Default
	// to the cloud's derived node id for this organization (same formula as NodeID with
	// the "cloud" mode), so a stock cloud needs no extra config; override with CLOUD_NODE_ID.
	cloudNodeID := os.Getenv("CLOUD_NODE_ID")
	if cloudNodeID == "" {
		cloudNodeID = uuid.NewSHA1(uuid.NameSpaceURL, []byte("laguna-escondida/node/"+organizationID+"/cloud")).String()
	}

	// SyncPushCron is how often the edge drains its outbox to the cloud (cron expr).
	syncPushCron := os.Getenv("SYNC_PUSH_CRON")
	if syncPushCron == "" {
		syncPushCron = "* * * * *"
	}

	// SyncPullCron is how often the edge pulls reference changes from the cloud (cron expr).
	syncPullCron := os.Getenv("SYNC_PULL_CRON")
	if syncPullCron == "" {
		syncPullCron = "* * * * *"
	}

	invoiceURLCron := os.Getenv("INVOICE_URL_CRON")
	if invoiceURLCron == "" {
		invoiceURLCron = "0 * * * *"
	}

	supportDocumentURLCron := os.Getenv("SUPPORT_DOCUMENT_URL_CRON")
	if supportDocumentURLCron == "" {
		supportDocumentURLCron = "30 * * * *"
	}

	return &Config{
		AppMode:                   appMode,
		NodeID:                    nodeID,
		NodeSyncKey:               nodeSyncKey,
		CloudSyncURL:              cloudSyncURL,
		CloudNodeID:               cloudNodeID,
		SyncPushCron:              syncPushCron,
		SyncPullCron:              syncPullCron,
		ElectronicInvoiceURL:      url,
		ElectronicInvoiceUser:     user,
		ElectronicInvoicePassword: password,
		ElectronicInvoicePrefix:   invoicePrefix,
		SupportDocumentPrefix:     supportDocumentPrefix,
		AdminAPIKey:               adminAPIKey,
		JWTSecret:                 jwtSecret,
		SpacesRegion:              spacesRegion,
		SpacesEndpoint:            spacesEndpoint,
		SpacesKey:                 spacesKey,
		SpacesSecret:              spacesSecret,
		SpacesBucket:              spacesBucket,
		OrganizationID:            organizationID,
		InvoiceURLCron:            invoiceURLCron,
		SupportDocumentURLCron:    supportDocumentURLCron,
	}, nil
}
