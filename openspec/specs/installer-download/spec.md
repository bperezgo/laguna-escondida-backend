# installer-download Specification

## Purpose
Gives a customer one stable place to get the box installer: a page on the landing site whose button always delivers the newest installer that works with the live cloud.

## Requirements

### Requirement: A public address redirects to the current installer

In the cloud, `GET /api/public/edge/installer` SHALL require no credential and SHALL answer `302 Found` with a `Location` header holding the download address of the current installer. The response SHALL carry `Cache-Control: no-store`, so a browser or proxy never keeps an old answer. The route SHALL NOT exist on an edge box.

#### Scenario: Download

- **WHEN** a visitor's browser requests `GET /api/public/edge/installer` and the current installer is `KuentaSetup-v1.4.0.exe`
- **THEN** the response is `302` with a `Location` ending in `/KuentaSetup-v1.4.0.exe`

#### Scenario: Called on an edge box

- **WHEN** `GET /api/public/edge/installer` is sent to an edge box
- **THEN** the response is `404 Not Found`

### Requirement: The current installer is the newest one the cloud can sync with

The current installer SHALL be the installer of the edge release with the highest version among releases that meet all of these: its tag is `edge/vX.Y.Z`; it is neither a draft nor a prerelease; its manifest can be read and lists an installer; and its manifest's `min_cloud` is at or below the version of the cloud answering the request. Versions SHALL be compared numerically by major, minor and patch. A cloud whose own version is not of the form `vX.Y.Z` (a development build) SHALL skip the `min_cloud` condition. Releases of the cloud and of the landing SHALL never be considered.

#### Scenario: Newest release needs a newer cloud

- **WHEN** the cloud runs `v1.3.0`, `edge/v1.5.0` declares `min_cloud` `v1.4.0`, and `edge/v1.4.0` declares `min_cloud` `v1.3.0`
- **THEN** the redirect points at the installer of `edge/v1.4.0`

#### Scenario: Prerelease is skipped

- **WHEN** `edge/v1.5.0` is marked as a prerelease and `edge/v1.4.0` is not
- **THEN** the redirect points at the installer of `edge/v1.4.0`

#### Scenario: Release without an installer

- **WHEN** the newest edge release has no installer in its manifest and the one before it has
- **THEN** the redirect points at the older release's installer

#### Scenario: Numeric ordering

- **WHEN** `edge/v1.10.0` and `edge/v1.9.0` are both eligible
- **THEN** the redirect points at the installer of `edge/v1.10.0`

#### Scenario: Other channels

- **WHEN** the newest release in the repository is `landing/v2.0.0`
- **THEN** it is ignored

### Requirement: No eligible installer is reported, not guessed

When no release is eligible, or the release list has never been read successfully, the endpoint SHALL answer `503 Service Unavailable` with a JSON body carrying `error` `installer_unavailable` and a `Retry-After` header. It SHALL NOT redirect anywhere.

#### Scenario: No edge release has an installer yet

- **WHEN** no edge release lists an installer
- **THEN** the response is `503` with `error` `installer_unavailable`

### Requirement: Downloads keep working when the release source is slow or down

The cloud SHALL reuse its knowledge of the releases for up to ten minutes, so that visitors' requests do not each cause a call to the release source, and a newly published release SHALL be offered within ten minutes. When refreshing fails, the cloud SHALL keep answering with the last current installer it determined, for as long as the failure lasts. A request SHALL NOT wait more than ten seconds on the release source.

#### Scenario: Many downloads

- **WHEN** fifty visitors request the installer within one minute
- **THEN** the release source is read at most once for them

#### Scenario: Release source is down

- **WHEN** the release source stops answering after the cloud has determined the current installer
- **THEN** requests keep receiving the redirect to that installer

#### Scenario: New release

- **WHEN** a newer eligible edge release is published
- **THEN** within ten minutes the redirect points at its installer

### Requirement: The redirect only leads to the project's release downloads

The endpoint SHALL redirect only to an `https` address under the download path of the project's configured release repository. A manifest that names any other address for its installer SHALL make that release ineligible.

#### Scenario: Manifest names a foreign address

- **WHEN** the newest release's manifest gives `https://example.com/KuentaSetup.exe` as its installer address
- **THEN** that release is skipped and no response redirects to `example.com`

### Requirement: The endpoint is limited per address

Requests from one network address above the limit SHALL be answered `429 Too Many Requests` without consulting the release source. The limit SHALL allow at least ten downloads from one address in ten minutes, so a restaurant behind one connection is not blocked by retries.

#### Scenario: Repeated requests

- **WHEN** one address sends requests far above the limit
- **THEN** the excess requests receive `429`

#### Scenario: A few retries

- **WHEN** one address requests the installer five times in a minute
- **THEN** all five receive the redirect

### Requirement: Each download is recorded without personal data

Each redirect SHALL be logged with the version served. The record SHALL NOT include the visitor's name, email or any identifier beyond what the cloud already logs for every request.

#### Scenario: Version in the log

- **WHEN** a request is redirected to the `v1.4.0` installer
- **THEN** the cloud's log has an entry for that download naming `v1.4.0`

### Requirement: The landing site has a download page

The landing site SHALL serve a page at `/descargar`, in Spanish, that contains: what the PC needs (Windows 10 or 11, 64-bit, an internet connection, the computer that stays at the restaurant); the three steps download, run, sign in with the restaurant's account; one download button; what to do when Windows shows its protection warning; and, for a visitor without an account, a link to the demo request on the home page. The page SHALL state that an account is needed to finish the installation. It SHALL NOT state a version, a file size or an installation time, and SHALL NOT contain testimonials, customer names or prices.

#### Scenario: Customer with an account

- **WHEN** a customer opens `/descargar` and presses the download button
- **THEN** the browser downloads the installer

#### Scenario: Visitor without an account

- **WHEN** a visitor without an account opens `/descargar`
- **THEN** the page tells them an account is needed and offers the demo request

### Requirement: The download button points at the configured cloud

The download button SHALL be a plain link to `/api/public/edge/installer` on the backend address the site was built with. The page SHALL work with scripts disabled. The address SHALL come from the site's single configuration module, never from a page or component.

#### Scenario: Scripts disabled

- **WHEN** a visitor with JavaScript disabled presses the download button
- **THEN** the download starts

#### Scenario: Built for another cloud

- **WHEN** the site is built with a different backend address
- **THEN** the button points at that backend, with no change to the page's source

### Requirement: The home page keeps one primary action

The home page SHALL keep "Pide una demo" as its only primary action: the download page SHALL be linked from the footer of every page and SHALL NOT appear in the header, the hero or as a button in any home section. The home page SHALL NOT say that the operator installs the equipment at the restaurant.

#### Scenario: Footer link

- **WHEN** a visitor opens the home page or the privacy page
- **THEN** the footer has a link to `/descargar`

#### Scenario: Header unchanged

- **WHEN** a visitor opens any page
- **THEN** the header's only action is "Pide una demo", and from `/descargar` it leads to the demo form on the home page

### Requirement: A download click is tracked without personal data

Pressing the download button SHALL send one event to each configured analytics tool through the site's single tracking module. The event SHALL carry no name, restaurant, phone or email. A blocked, missing or failing analytics tool SHALL NOT stop or delay the download.

#### Scenario: Analytics configured

- **WHEN** a visitor presses the download button on a site with analytics configured
- **THEN** one download event is sent and the download starts

#### Scenario: Analytics blocked

- **WHEN** the visitor's browser blocks the analytics scripts
- **THEN** the download starts as usual
