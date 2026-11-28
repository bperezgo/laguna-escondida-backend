# Spec Delta

## Purpose

Defines the local name and address an edge box serves the POS on, so devices in the restaurant reach it by a name the gateway resolves, and how that identity is kept correct when it changes on an installed box.

## ADDED Requirements

### Requirement: A box serves the POS on its local name and its IP

A box SHALL serve the POS over HTTPS on every local name its configuration lists and on its LAN IP, each with a certificate from the box's internal CA whose subject alternative names hold that name or the IP. A box configured with no name SHALL serve on its IP only. The box SHALL NOT answer DNS queries; the gateway resolves the name.

#### Scenario: Name and IP both work

- **GIVEN** a box configured with IP `192.168.1.50` and name `pos.kuenta.lan`
- **WHEN** a tablet that trusts the box's root CA opens `https://pos.kuenta.lan` (resolved by the gateway to `192.168.1.50`) and `https://192.168.1.50`
- **THEN** both load the POS with no certificate warning

#### Scenario: Box without a name

- **GIVEN** a box configured with IP `192.168.1.50` and no name
- **WHEN** a tablet opens `https://192.168.1.50`
- **THEN** the POS loads, and the certificate lists only `192.168.1.50`

#### Scenario: The box is not a DNS server

- **WHEN** a device sends a DNS query for `pos.kuenta.lan` to the box's IP
- **THEN** the box does not answer it

### Requirement: Only the configured network reaches the box

A box SHALL accept HTTPS and HTTP connections only from addresses inside its configured network (CIDR), both in the Windows Firewall and in the proxy. The configured network SHALL be inside a private IPv4 range (`10.0.0.0/8`, `172.16.0.0/12` or `192.168.0.0/16`), with a prefix of `/16` or longer, and SHALL contain the box's IP.

#### Scenario: Device outside the network

- **GIVEN** a box configured with network `192.168.1.0/24`
- **WHEN** a device at `192.168.2.10` connects to port 443
- **THEN** the device does not get the POS: the firewall drops the connection, or the proxy answers `403`

#### Scenario: Network wider than allowed is rejected

- **WHEN** the box is configured with network `0.0.0.0/0`, `192.168.0.0/8` or `8.8.8.0/24`
- **THEN** the configuration is rejected with a message naming the network, and no firewall rule allowing it is created

### Requirement: Changing an installed box's IP keeps its name working

When the LAN IP of an installed box is changed with the box's IP-change tool, the box SHALL serve afterwards on its configured names and the new IP, with certificates whose subject alternative names hold the new IP and each name. It SHALL no longer serve on the old IP. The configured names and the internal root CA SHALL NOT change, so devices that already trust the box need no new certificate.

#### Scenario: IP changes, name keeps working

- **GIVEN** a box serving `pos.kuenta.lan` on `192.168.1.50`
- **WHEN** its IP is changed to `192.168.1.60` and the gateway record is updated to `192.168.1.60`
- **THEN** `https://pos.kuenta.lan` and `https://192.168.1.60` load the POS with no certificate warning on an already provisioned tablet
- **AND** the certificates the box serves hold `192.168.1.60` and `pos.kuenta.lan`, and none holds `192.168.1.50`
