package dto

// The edge update DTOs mirror the JSON files the appliance updater shares with this node
// (laguna-escondida-edge docs/UPDATE_PROTOCOL.md). Times stay strings: they are passed through as written.

type EdgeInstalledRelease struct {
	Version   string `json:"version"`
	AppliedAt string `json:"applied_at"`
}

type EdgeUpdateAvailable struct {
	Version     string            `json:"version"`
	Current     string            `json:"current"`
	PublishedAt string            `json:"published_at"`
	NotesURL    string            `json:"notes_url"`
	Components  map[string]string `json:"components"`
	CheckedAt   string            `json:"checked_at"`
}

type EdgeUpdateRequest struct {
	Version     string `json:"version"`
	RequestedBy string `json:"requested_by"`
	RequestedAt string `json:"requested_at"`
}

type EdgeUpdateProgress struct {
	Version   string `json:"version"`
	Phase     string `json:"phase"`
	StartedAt string `json:"started_at"`
}

type EdgeUpdateResult struct {
	Version    string            `json:"version"`
	Outcome    string            `json:"outcome"`
	Message    string            `json:"message"`
	StartedAt  string            `json:"started_at"`
	FinishedAt string            `json:"finished_at"`
	From       map[string]string `json:"from"`
	To         map[string]string `json:"to"`
}

// EdgeUpdateState holds one entry per state file; nil when that file is missing or unreadable.
type EdgeUpdateState struct {
	Installed  *EdgeInstalledRelease
	Available  *EdgeUpdateAvailable
	Requested  *EdgeUpdateRequest
	InProgress *EdgeUpdateProgress
	LastResult *EdgeUpdateResult
}
