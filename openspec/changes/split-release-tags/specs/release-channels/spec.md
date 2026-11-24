## Purpose

Defines the release channels (cloud, edge, landing), what each tag builds, and how a box decides which edge release to install. It keeps cloud-only changes from reaching the restaurant boxes while never leaving a box newer than the cloud it syncs with.

## ADDED Requirements

### Requirement: Each channel has its own tag prefix
The system SHALL release through three tag prefixes: `cloud/v*`, `edge/v*` and `landing/v*`. A tag SHALL build only the artifacts of its own channel. A tag named `v*` without a prefix SHALL NOT build or publish anything.

#### Scenario: Cloud tag builds cloud artifacts only
- **WHEN** a `cloud/v*` tag is pushed
- **THEN** the backend image and the web image are built and pushed to ECR with the tag's version
- **AND** no appliance artifact or GitHub Release is created

#### Scenario: Edge tag builds the appliance release only
- **WHEN** an `edge/v*` tag is pushed
- **THEN** one GitHub Release named after the tag is published with the appliance zip, the edge-node Windows zip, the Next.js Windows bundle zip, a sha256 file for each, and `manifest.json`
- **AND** no image is pushed to ECR

#### Scenario: Landing tag builds the landing site only
- **WHEN** a `landing/v*` tag is pushed
- **THEN** only the landing site is built and released

#### Scenario: Unprefixed tag does nothing
- **WHEN** a tag `v1.2.6` is pushed
- **THEN** no release workflow publishes any artifact

### Requirement: The manifest describes the whole appliance release
Each edge release SHALL include a `manifest.json` that lists every component with its version, download URL and sha256, and that states `min_cloud`: the lowest cloud version the release can sync with. All components SHALL be built from the same commit.

#### Scenario: Manifest is complete
- **WHEN** an edge release is published
- **THEN** its manifest lists the appliance, edge-node and Next.js components, each with a URL and sha256 matching the attached files
- **AND** it carries a `min_cloud` that is an existing `cloud/v*` version

### Requirement: A box is offered only a compatible edge release
A box's updater SHALL consider only `edge/v*` releases. It SHALL offer the newest one that is newer than what is installed and whose `min_cloud` is less than or equal to the version the live cloud reports. It SHALL install edge-node and Next.js together as the pair from that release. A release it cannot verify by sha256 SHALL NOT be offered.

#### Scenario: Cloud release leaves the box alone
- **WHEN** a `cloud/v*` tag is deployed and the box checks for updates
- **THEN** nothing new is offered, because no edge release was created

#### Scenario: Edge release the cloud is not ready for
- **WHEN** the newest edge release has a `min_cloud` above the version the cloud reports
- **THEN** that release is not offered
- **AND** the newest older release that is compatible is offered if it is newer than what is installed

#### Scenario: Compatible edge release is offered
- **WHEN** a newer edge release exists whose `min_cloud` the cloud satisfies and its files verify
- **THEN** the box offers it, and installing it replaces edge-node and Next.js with that release's pair

### Requirement: An unchanged edge is not re-released
CI SHALL compute an edge fingerprint from every input of the appliance release: the sources of the packages the edge binary compiles, its embedded migrations, the frontend sources, the appliance files under `edge/`, the dependency lockfiles and the build scripts. Each edge release SHALL record its fingerprint. CI SHALL report whether the current fingerprint equals the last released one.

#### Scenario: Edge-irrelevant change keeps the fingerprint
- **WHEN** a change touches only code that no appliance artifact is built from
- **THEN** the fingerprint equals the last edge release's
- **AND** CI reports that no edge release is needed

#### Scenario: Shared code changes the fingerprint
- **WHEN** a change touches a package the edge binary compiles, the frontend, or a file under `edge/`
- **THEN** the fingerprint differs from the last edge release's
- **AND** CI reports that an edge release is needed

#### Scenario: Edge tag with an unchanged fingerprint is refused
- **WHEN** an `edge/v*` tag is pushed and the fingerprint equals the last release's
- **THEN** the release fails without publishing, and says the edge is unchanged

### Requirement: Cloud accepts the previous edge release
The edge SHALL send its release version on every sync request. The cloud SHALL accept an edge on the previous `edge/v*` release, so a cloud release never forces an edge release, and SHALL reject one below its configured minimum.

#### Scenario: Newer cloud, previous edge
- **WHEN** the cloud runs a release newer than the edge needs and the edge pushes or pulls sync data
- **THEN** the cloud serves it as before

#### Scenario: Edge too old
- **WHEN** an edge below the cloud's configured minimum contacts the cloud
- **THEN** the cloud rejects the sync with an error that names the minimum supported edge release

#### Scenario: Edge that sends no version
- **WHEN** an edge that predates the version header contacts the cloud
- **THEN** the cloud serves it, so existing boxes keep syncing until the minimum is first raised
