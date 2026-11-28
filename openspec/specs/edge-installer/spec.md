# edge-installer Specification

## Purpose
Lets a restaurant owner put the edge box on their own Windows PC by running one file and signing in, with no terminal, no file editing and no help from the operator.

## Requirements

### Requirement: One file installs the whole appliance

The installer SHALL be a single executable, `KuentaSetup-<version>.exe`, that installs every part of the appliance (database, edge node, POS web app, LAN proxy, their Windows services and the updater task) on a 64-bit Windows 10 or Windows 11 PC. The owner SHALL need only to run it, approve the Windows elevation prompt, and sign in. Its screens and messages SHALL be in Spanish. It SHALL install to `C:\laguna-edge` and SHALL NOT ask for a folder. It SHALL NOT require the owner to open a terminal or edit a file.

#### Scenario: Fresh PC

- **WHEN** an admin of an organization with no box runs the installer on a supported PC and signs in
- **THEN** the wizard finishes without the owner typing anything but their email and password
- **AND** every appliance service is running and the POS answers on the PC

#### Scenario: Installed version

- **WHEN** `KuentaSetup-v1.4.0.exe` finishes
- **THEN** the box reports version `v1.4.0` for the edge node and the POS web app

### Requirement: The installer checks the PC before changing it

Before writing anything outside its own temporary files, the installer SHALL check that: Windows is 64-bit Windows 10 or 11; it is running elevated; at least 3 GB are free on drive `C:`; none of the ports 80, 443, 3000, 5432 and 8080 is held by a program that is not part of an existing appliance install; and the cloud answers. When a check fails it SHALL stop, change nothing, and show one message that names the failed check in plain Spanish. For a busy port the message SHALL name the port and the program holding it.

#### Scenario: Port in use

- **WHEN** another program is listening on port 5432 and the owner runs the installer
- **THEN** the wizard stops before installing, naming port 5432 and that program
- **AND** `C:\laguna-edge` does not exist afterwards

#### Scenario: No connection to the cloud

- **WHEN** the PC cannot reach the cloud
- **THEN** the wizard stops before asking for credentials and says there is no connection

#### Scenario: Unsupported Windows

- **WHEN** the installer runs on 32-bit Windows or on a version older than Windows 10
- **THEN** it stops with a message saying which Windows is required

### Requirement: The owner signs in and the box is enrolled

The installer SHALL ask for the owner's cloud email and password, sign in to the cloud with them, and enroll the box under the name of the PC. It SHALL tell apart, each with its own message: wrong email or password; a user who is not an admin of their organization; and a connection failure. After any of them the owner SHALL be able to try again without restarting the wizard. The password SHALL be held only in memory: it SHALL NOT be written to any file or log, and SHALL NOT appear in the arguments of any process. The user token SHALL be discarded when enrollment ends.

#### Scenario: Wrong password

- **WHEN** the owner types a wrong password
- **THEN** the wizard says the email or password is wrong and stays on the sign-in screen
- **AND** nothing is installed

#### Scenario: Not an admin

- **WHEN** a manager signs in
- **THEN** the wizard says only an administrator of the restaurant can install the box

#### Scenario: Password leaves no trace

- **WHEN** an install finishes
- **THEN** no file under `C:\laguna-edge` or the installer's log contains the password or the user token

### Requirement: Replacing an existing box needs the owner's confirmation

When the cloud answers that the organization already has an active box, the installer SHALL show that box's name and when it last synced (or that it never has), SHALL warn that anything that box has not synced will be lost, and SHALL enroll with replacement only after the owner confirms. Declining SHALL end the wizard with nothing installed and the existing box untouched.

#### Scenario: Owner confirms

- **WHEN** the organization has an active box "Caja principal" last seen yesterday, and the owner confirms the replacement
- **THEN** the new PC is enrolled and the wizard continues

#### Scenario: Owner declines

- **WHEN** the owner declines the replacement
- **THEN** the wizard ends, nothing is installed, and "Caja principal" keeps syncing

### Requirement: The installer writes the box's configuration

The installer SHALL write the box's configuration itself:
- network: the PC's IPv4 address on the network that reaches the internet, and that network's subnet
- identity: the organization id, node id, node key and cloud address exactly as enrollment returned them
- database: a password of at least 128 random bits, generated on the PC

No configuration file SHALL be left with a placeholder value. The files holding the node key and the database password SHALL be readable only by the Administrators group and the system account. Ticket printing from the box SHALL be explicitly turned off, so the POS prints through the browser; a box with printing off SHALL NOT accept a print request and SHALL NOT write tickets anywhere.

#### Scenario: Printing is off

- **WHEN** a freshly installed box receives `POST /api/device/print` from a signed-in manager
- **THEN** the response is `404 Not Found` and no ticket file is written on the PC

#### Scenario: Values come from enrollment

- **WHEN** enrollment returns node id N, a node key and `cloud_sync_url` `https://api.example.com`
- **THEN** the box runs as node N and syncs against `https://api.example.com`

#### Scenario: Detected network

- **WHEN** the PC holds `192.168.1.40` with mask `255.255.255.0` on its internet-facing network
- **THEN** the box serves the LAN on `192.168.1.40` and accepts connections from `192.168.1.0/24`

#### Scenario: Secrets are not world-readable

- **WHEN** a standard (non-administrator) Windows user tries to read the box's configuration files
- **THEN** access is denied

### Requirement: An install downloads nothing from third parties

The installer SHALL contain the database server, the LAN proxy, the JavaScript runtime, the telemetry sidecar and the service wrapper at the versions pinned for that release, together with the edge node and POS web app of that release. During an install the PC SHALL contact only the cloud. Each third-party component SHALL have been checked against a pinned sha256 when the installer was built, and a mismatch SHALL fail the build. The installer SHALL include the licence text of every third-party component it carries.

#### Scenario: Third-party sites unreachable

- **WHEN** the PC can reach the cloud but not GitHub, nodejs.org or the database vendor's site
- **THEN** the install completes

#### Scenario: Tampered component at build time

- **WHEN** the installer is built and a downloaded third-party archive does not match its pinned sha256
- **THEN** the build fails and no installer is produced

### Requirement: The wizard finishes when the box is usable

After registering the services the installer SHALL wait until the box serves the POS and has completed a sync with the cloud, so that the restaurant's users can sign in on the box. It SHALL show progress while it waits. When the box serves but has not synced within 5 minutes, the wizard SHALL finish with a warning that sign-in will work once the box reaches the cloud; it SHALL NOT report the install as failed. On finishing, it SHALL show the organization's name and offer to open the POS.

#### Scenario: Synced before finishing

- **WHEN** the wizard shows its final screen without a warning
- **THEN** a user of the organization can sign in on the box

#### Scenario: Cloud drops during the wait

- **WHEN** the box is serving but the cloud becomes unreachable right after enrollment
- **THEN** after 5 minutes the wizard finishes with the warning, and the box syncs by itself when the connection returns

### Requirement: The POS opens from a Kuenta shortcut

The installer SHALL create a shortcut named "Kuenta" on the desktop and in the Start menu, with the product's icon, that opens the box's POS in a window of its own, without a browser address bar or tabs. On a PC where Microsoft Edge is not available the shortcut SHALL open the POS in the default browser. The POS window SHALL show the product's name and icon as its title and taskbar icon.

#### Scenario: Opening the POS

- **WHEN** the owner double-clicks "Kuenta" on the desktop
- **THEN** the POS sign-in appears in its own window with no address bar

### Requirement: Running the installer on an existing box repairs it

When the PC already holds a box's configuration, the installer SHALL offer only a repair: it SHALL NOT ask for credentials, SHALL NOT enroll, and SHALL NOT change the box's identity, database or configuration; it SHALL register the services and the updater task again, restore the programs of the installer's version when they are older, and recreate the shortcut. When the PC holds a box's database but no configuration, the installer SHALL stop, change nothing, and tell the owner to contact support. The installer SHALL never delete or reinitialize an existing database.

#### Scenario: Repair keeps everything

- **WHEN** the installer runs on a working box and the owner chooses repair
- **THEN** the box has the same node id, the same data and the same sessions as before, and its services are running

#### Scenario: Install interrupted after enrollment

- **WHEN** a first install was interrupted after the configuration was written, and the owner runs the installer again
- **THEN** the install completes as a repair, without enrolling a second time

#### Scenario: Database without configuration

- **WHEN** `C:\laguna-edge` holds a database but no box configuration
- **THEN** the wizard stops and the database is untouched

#### Scenario: Older installer on a newer box

- **WHEN** `KuentaSetup-v1.4.0.exe` runs on a box the updater has already taken to `v1.5.0`
- **THEN** the box stays on `v1.5.0`

### Requirement: Uninstalling keeps the restaurant's data

Removing "Kuenta" from Windows SHALL stop and remove the appliance's services, its updater task, its firewall rule, its programs and its shortcuts. It SHALL keep the database, the box's configuration, the backups and the LAN certificate data, and SHALL say so on its last screen. It SHALL NOT change the box's enrollment in the cloud. Running the installer afterwards SHALL repair the box with its data intact.

#### Scenario: Uninstall then reinstall

- **WHEN** the owner uninstalls Kuenta and later runs the installer again
- **THEN** the orders and users that were on the box are still there, and no new enrollment happens

### Requirement: The installer can run unattended

The installer SHALL support a silent mode for automated tests and assisted installs, taking the email and password from environment variables, an explicit choice for replacement, and an optional cloud address that overrides the built-in one. In silent mode every failure SHALL end the process with a non-zero exit code and a line in the installer log naming the cause, and SHALL never wait for input.

#### Scenario: Silent install succeeds

- **WHEN** the installer runs silently with valid credentials against a cloud that enrolls it
- **THEN** it exits with code 0 and the box is serving

#### Scenario: Silent install needs a decision it was not given

- **WHEN** the installer runs silently, the organization already has a box, and replacement was not requested
- **THEN** it exits non-zero, logs that a box is already enrolled, and installs nothing

### Requirement: Each edge release publishes its installer

Every `edge/v*` release SHALL carry `KuentaSetup-<version>.exe` and its `.sha256`, built from the same commit and the same edge node and POS artifacts as the rest of that release, and `manifest.json` SHALL list it with its version, download address and sha256. The installer SHALL have the cloud's public address built in at release time. When a code-signing certificate is configured for the release, the installer and its uninstaller SHALL be signed and the release SHALL fail if signing fails; when none is configured, the installer SHALL be published unsigned and the release notes SHALL say so. A release whose installer fails to build or fails its install test SHALL NOT be published.

#### Scenario: Release assets

- **WHEN** `edge/v1.4.0` is released
- **THEN** the release has `KuentaSetup-v1.4.0.exe` and `KuentaSetup-v1.4.0.exe.sha256`
- **AND** `manifest.json` names the installer with the same sha256

#### Scenario: Installer matches the release

- **WHEN** `KuentaSetup-v1.4.0.exe` is installed
- **THEN** the edge node and POS it installs are byte-identical to the `edge-node` and `next` archives of `edge/v1.4.0`

#### Scenario: No certificate configured

- **WHEN** a release runs without a signing certificate
- **THEN** the installer is published unsigned and the release notes state it

#### Scenario: Installer test fails

- **WHEN** the built installer fails its silent install test
- **THEN** no `edge/v*` release is published
