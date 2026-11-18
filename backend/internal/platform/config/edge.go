package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/google/uuid"
)

// Edge is the edge box's configuration: one organization, one node identity.
type Edge struct {
	Shared

	OrganizationID string
	// NodeID is the box's one identity: X-Node-ID on sync and telemetry, and origin_node_id on
	// its outbox rows. It must match the box's row in the cloud's nodes table.
	NodeID string
	// CloudNodeID is the peer key the box's sync_state cursors are kept under.
	CloudNodeID string
	// CloudSyncURL is the cloud the sync loops target; when unset (or NodeSyncKey is unset)
	// the loops stay disabled rather than failing to boot.
	CloudSyncURL string

	SyncPushCron string
	SyncPullCron string
	// StockPullCron is daily and early, so each service day opens with the office's purchases
	// and counts in place and the displayed numbers stay stable through the shift.
	StockPullCron string

	Printer Printer
	Ticket  Ticket

	// LagunaStateDir is the appliance updater's state directory; empty turns owner-triggered updates off.
	LagunaStateDir string
}

// Printer configures POST /api/device/print. All optional with dev-friendly defaults
// (ESC/POS to a file) so a non-printing box boots unchanged.
type Printer struct {
	Transport string
	Target    string
	WidthMM   int
	Codepage  string
	Cut       string
}

type Ticket struct {
	BusinessName    string
	BusinessNIT     string
	BusinessAddress string
	Footer          string
	LegalNotice     string
}

func deriveNodeID(organizationID string, mode Mode) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("laguna-escondida/node/"+organizationID+"/"+string(mode))).String()
}

// CloudNodeIDFor is the cloud peer key an edge box of organizationID keeps its sync cursors under.
// It must never change: existing boxes' sync_state rows are keyed by it.
func CloudNodeIDFor(organizationID string) string {
	return deriveNodeID(organizationID, ModeCloud)
}

func LoadEdge() (*Edge, error) {
	shared, err := loadShared()
	if err != nil {
		return nil, err
	}

	organizationID := os.Getenv("ORGANIZATION_ID")
	if organizationID == "" {
		return nil, errors.New("ORGANIZATION_ID is required on the edge")
	}
	if uuid.Validate(organizationID) != nil {
		return nil, fmt.Errorf("ORGANIZATION_ID %q is not an organization id (a UUID)", organizationID)
	}

	nodeID := os.Getenv("NODE_ID")
	if nodeID == "" {
		return nil, errors.New("NODE_ID is required on the edge")
	}
	if uuid.Validate(nodeID) != nil {
		return nil, fmt.Errorf("NODE_ID %q is not a node id (a UUID)", nodeID)
	}

	stockPullCron := envOr("STOCK_PULL_CRON", "0 5 * * *")
	if err = validateCronExpression("STOCK_PULL_CRON", stockPullCron); err != nil {
		return nil, err
	}

	printer, err := loadPrinter()
	if err != nil {
		return nil, err
	}

	return &Edge{
		Shared:         shared,
		OrganizationID: organizationID,
		NodeID:         nodeID,
		CloudNodeID:    CloudNodeIDFor(organizationID),
		CloudSyncURL:   os.Getenv("CLOUD_SYNC_URL"),
		SyncPushCron:   envOr("SYNC_PUSH_CRON", "* * * * *"),
		SyncPullCron:   envOr("SYNC_PULL_CRON", "* * * * *"),
		StockPullCron:  stockPullCron,
		Printer:        printer,
		Ticket: Ticket{
			BusinessName:    os.Getenv("BUSINESS_NAME"),
			BusinessNIT:     os.Getenv("BUSINESS_NIT"),
			BusinessAddress: os.Getenv("BUSINESS_ADDRESS"),
			Footer:          os.Getenv("TICKET_FOOTER"),
			LegalNotice:     os.Getenv("TICKET_LEGAL_NOTICE"),
		},
		LagunaStateDir: os.Getenv("LAGUNA_STATE_DIR"),
	}, nil
}

func loadPrinter() (Printer, error) {
	widthMM := 80
	if v := os.Getenv("PRINTER_WIDTH_MM"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return Printer{}, fmt.Errorf("invalid PRINTER_WIDTH_MM %q: %w", v, err)
		}
		widthMM = parsed
	}
	return Printer{
		Transport: envOr("PRINTER_TRANSPORT", "file"),
		Target:    os.Getenv("PRINTER_TARGET"),
		WidthMM:   widthMM,
		Codepage:  envOr("PRINTER_CODEPAGE", "CP850"),
		Cut:       envOr("PRINTER_CUT", "partial"),
	}, nil
}
