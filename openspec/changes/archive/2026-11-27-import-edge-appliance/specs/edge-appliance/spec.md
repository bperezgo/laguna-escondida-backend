## Purpose

Defines where the edge appliance (services, proxy, updater, install scripts) lives in the monorepo and what protects it. It keeps the appliance reviewable and tested alongside the apps it runs.

## ADDED Requirements

### Requirement: The appliance lives in `edge/`
The appliance's scripts, service definitions, proxy config, telemetry sidecar config, provisioning docs and incident notes SHALL live under `edge/` in this repository, with their commit history preserved. The old standalone repository SHALL be read-only and point here.

#### Scenario: History is preserved
- **WHEN** the history of a file under `edge/` is listed
- **THEN** it includes the commits made in the standalone repository

#### Scenario: Old repository points here
- **WHEN** someone opens the standalone `laguna-escondida-edge` repository
- **THEN** it is archived and its README names this repository's `edge/` folder

### Requirement: No per-box secrets or runtime state are committed
Per-box environment files, node keys, certificates, fetched binaries, logs, backups and runtime state SHALL NOT be tracked under `edge/`. Only `*.example` templates SHALL be committed.

#### Scenario: Ignored paths stay untracked
- **WHEN** the tracked files under `edge/` are listed
- **THEN** none is a per-box `env/*.env`, `env/alloy-node-key`, a certificate, a fetched artifact, a log, a backup or a state file

#### Scenario: History is clean before import
- **WHEN** the standalone repository's full history is scanned for credentials before the import
- **THEN** no finding is left unresolved: each is either a false positive or its secret has been rotated

### Requirement: CI proves the appliance installs
A change under `edge/**` SHALL run the Windows install smoke test: fetch, install, verify, re-install, update, owner-triggered update and uninstall on a clean machine. A change that touches no `edge/**` path SHALL NOT run it.

#### Scenario: Appliance change is smoke-tested
- **WHEN** a pull request changes a file under `edge/` other than documentation
- **THEN** the install smoke test runs and must pass to merge

#### Scenario: Unrelated change skips it
- **WHEN** a pull request changes only `backend/`, `frontend/` or `landing/`
- **THEN** the install smoke test does not run

### Requirement: The appliance consumes apps only through their artifacts
`edge/` SHALL NOT import source code from `backend/` or `frontend/`. It SHALL consume the edge-node binary and the frontend bundle only as built release artifacts, referenced by tag and verified by sha256.

#### Scenario: No source imports
- **WHEN** the files under `edge/` are inspected for references into `backend/` or `frontend/` source
- **THEN** there are none; app code is reached only by downloading a release artifact
