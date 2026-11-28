## MODIFIED Requirements

### Requirement: Uninstalling keeps the restaurant's data

Removing "Kuenta" from Windows SHALL stop and remove the appliance's services, its updater task, its firewall rule, its programs and its shortcuts. Unless the owner chooses to remove the box's data, it SHALL keep the database, the box's configuration, the backups and the LAN certificate data, and running the installer afterwards SHALL repair the box with its data intact. It SHALL NOT change the box's enrollment in the cloud.

#### Scenario: Uninstall then reinstall

- **WHEN** the owner uninstalls Kuenta, keeps the data, and later runs the installer again
- **THEN** the orders and users that were on the box are still there, and no new enrollment happens

#### Scenario: Keeping the data is the default

- **WHEN** the owner uninstalls Kuenta and accepts the question's default answer
- **THEN** the box's database and configuration are still on the PC

## ADDED Requirements

### Requirement: The uninstall asks before removing the box's data

Before it removes anything, the uninstall SHALL ask the owner whether to also remove the box's data, warning that what the box has not synced to the cloud is lost. Keeping the data SHALL be the default answer. A silent uninstall SHALL keep the data unless it is started with `/PURGE`.

#### Scenario: Silent uninstall keeps the data

- **WHEN** the uninstaller runs with `/VERYSILENT /SUPPRESSMSGBOXES` and without `/PURGE`
- **THEN** the box's database and configuration are still on the PC

#### Scenario: Silent uninstall with /PURGE

- **WHEN** the uninstaller runs with `/VERYSILENT /SUPPRESSMSGBOXES /PURGE`
- **THEN** everything is removed as when the owner answers yes

### Requirement: Removing the box's data leaves a clean PC

When the owner chooses to remove the data, the uninstall SHALL also remove the install folder, the box's own root certificate from the PC's trusted certificates and the machine settings the appliance created, so the next install is a fresh one. It SHALL NOT remove any other trusted certificate.

#### Scenario: Owner removes everything

- **WHEN** the owner uninstalls Kuenta and answers yes to removing the data
- **THEN** the install folder no longer exists, the box's root certificate is no longer trusted on the PC, and none of the appliance's services, task, firewall rule, shortcuts or machine settings remain
- **AND** Kuenta no longer appears in the PC's installed apps

#### Scenario: Reinstall after removing everything

- **GIVEN** the owner removed everything on uninstall
- **WHEN** the owner runs the installer again with a valid code for an organization that still has this box enrolled
- **THEN** the installer runs as a fresh install, asks whether to replace the existing box, and on yes enrolls a new box

#### Scenario: Other trusted certificates are untouched

- **GIVEN** the PC trusts another root certificate whose name also contains `Caddy Local Authority`
- **WHEN** the owner uninstalls Kuenta and removes everything
- **THEN** that other certificate is still trusted

### Requirement: The uninstall's last screen says what it did

The uninstall's last screen SHALL say whether the box's data was kept, removed, or could not be removed completely.

#### Scenario: Last screen after keeping the data

- **WHEN** the owner uninstalls Kuenta and keeps the data
- **THEN** the last screen says the data was kept in `C:\kuentapos`

#### Scenario: Last screen after removing everything

- **WHEN** the owner uninstalls Kuenta, removes everything, and every file could be deleted
- **THEN** the last screen says the data was removed and the next install starts from scratch
