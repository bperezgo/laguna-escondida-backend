## ADDED Requirements

### Requirement: The owner can set the box's network before enrolling

On a fresh install the welcome screen SHALL offer an unticked option to set the network manually. Only when it is ticked, a network screen SHALL come before sign-in, showing the box's IP (a choice among the IPv4 addresses the PC holds, the detected one preselected), the allowed network (derived from the chosen address, editable) and the local name (`pos.kuenta.lan`, editable). Invalid values SHALL keep the owner on that screen with a message, before the code is used.

#### Scenario: Advanced option not ticked

- **WHEN** the owner leaves the option unticked and continues
- **THEN** the wizard goes from the welcome screen to sign-in without showing the network screen
- **AND** the box is configured with the detected IP, its network and the name `pos.kuenta.lan`

#### Scenario: Owner picks another address

- **GIVEN** the PC holds `192.168.1.50` on Ethernet and `10.0.0.7` on Wi-Fi, and the default route is on Wi-Fi
- **WHEN** the owner ticks the option, picks `192.168.1.50` and continues
- **THEN** the allowed network shown is `192.168.1.0/24`
- **AND** the box serves on `192.168.1.50` and accepts connections from `192.168.1.0/24`

#### Scenario: Only addresses the PC holds can be chosen

- **WHEN** the network screen is shown
- **THEN** the IP choice lists only IPv4 addresses the PC holds, excluding `127.0.0.0/8` and `169.254.0.0/16`, and does not accept typed text

#### Scenario: Network that does not contain the IP

- **WHEN** the owner chooses `192.168.1.50` and types the network `192.168.2.0/24`
- **THEN** the wizard stays on the network screen and says the IP is not inside the network
- **AND** the installation code has not been sent to the cloud

#### Scenario: Network outside the private ranges

- **WHEN** the owner types the network `0.0.0.0/0`
- **THEN** the wizard stays on the network screen and says the network must be a private network of at most 65,536 addresses

#### Scenario: Invalid name

- **WHEN** the owner types the name `pos kuenta` or `-pos.lan`
- **THEN** the wizard stays on the network screen and says the name is not valid

#### Scenario: Name left blank

- **WHEN** the owner clears the name and continues
- **THEN** the box serves on its IP only, and the final screen shows only `https://<ip>` and no gateway record

#### Scenario: Repair does not show the network screen

- **WHEN** the installer runs on a PC that already holds a box's configuration
- **THEN** neither the option nor the network screen is shown, and the box's IP, network and name do not change

### Requirement: A silent install takes the box's network

In silent mode the installer SHALL take the box's IP, allowed network and local name from the optional environment variables `KUENTA_SETUP_LAN_IP`, `KUENTA_SETUP_LAN_CIDR` and `KUENTA_SETUP_LAN_HOST` (`none` for no name). Each one left unset SHALL get its default. The values SHALL pass the same checks as on the network screen before the code is sent, and a failure SHALL end the install non-zero with the cause in the log.

#### Scenario: Silent install with a name

- **WHEN** the installer runs silently with `KUENTA_SETUP_LAN_HOST=caja.kuenta.lan` and the IP and network unset
- **THEN** the box serves on the detected IP and on `caja.kuenta.lan`

#### Scenario: Silent install with an IP the PC does not hold

- **WHEN** the installer runs silently with `KUENTA_SETUP_LAN_IP=192.168.77.77` on a PC that does not hold it
- **THEN** it exits non-zero, logs that the PC does not hold `192.168.77.77`, the cloud receives no enrollment call, and nothing is installed

### Requirement: The final screen names the box's addresses

When an install or repair finishes without failing, the final screen SHALL show the addresses devices use to open the POS (`https://<name>` and `https://<ip>`, or only `https://<ip>` for a box without a name). It SHALL also name what the gateway must hold: the DNS record `<name> -> <ip>` when the box has a name, and a fixed IP for the PC.

#### Scenario: Final screen names the gateway record

- **WHEN** a fresh install on `192.168.1.50` with name `pos.kuenta.lan` finishes
- **THEN** the final screen shows `https://pos.kuenta.lan`, `https://192.168.1.50` and the record `pos.kuenta.lan -> 192.168.1.50`

## MODIFIED Requirements

### Requirement: The installer writes the box's configuration

The installer SHALL write the box's configuration itself:
- network: the IPv4 address and allowed network the owner confirmed on the network screen, or, when the owner did not open it, the PC's IPv4 address on the network that reaches the internet and that network's subnet
- local name: the name the owner confirmed on the network screen, or `pos.kuenta.lan`
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

- **WHEN** the PC holds `192.168.1.40` with mask `255.255.255.0` on its internet-facing network and the owner did not open the network screen
- **THEN** the box serves the LAN on `192.168.1.40` and on `pos.kuenta.lan`, and accepts connections from `192.168.1.0/24`

#### Scenario: Confirmed network

- **WHEN** the owner confirmed IP `192.168.1.50`, network `192.168.1.0/24` and name `caja.kuenta.lan` on the network screen
- **THEN** the box serves the LAN on `192.168.1.50` and on `caja.kuenta.lan`, and accepts connections from `192.168.1.0/24`

#### Scenario: Secrets are not world-readable

- **WHEN** a standard (non-administrator) Windows user tries to read the box's configuration files
- **THEN** access is denied
