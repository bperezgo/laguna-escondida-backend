package dto

// EdgeRelease is a published appliance release: its tag is edge/<Version>.
type EdgeRelease struct {
	Version    string
	Draft      bool
	Prerelease bool
}

// EdgeManifest is what an edge release's manifest.json says. InstallerURL is empty when it lists no installer.
type EdgeManifest struct {
	MinCloud     string
	InstallerURL string
}

// EdgeInstaller is the installer a visitor is sent to.
type EdgeInstaller struct {
	Version string
	URL     string
}
