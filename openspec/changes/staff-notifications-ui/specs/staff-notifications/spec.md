## MODIFIED Requirements

### Requirement: Staff manage notification channels

The cloud SHALL serve `GET`, `POST /api/notification-channels` and `GET`,
`PUT`, `DELETE /api/notification-channels/:id`, scoped to the caller's
organization. Reading SHALL require `notification-channels:read`, granted
to admins and managers. Changing SHALL require
`notification-channels:manage`, granted only to admins. A channel SHALL
have a `name` (1–80 characters, unique within the organization), a `type`,
an `enabled` flag (default `true`) and the settings of its type. The only
type SHALL be `discord`, whose setting is `webhook_url`. It SHALL be an
`https` URL on `discord.com` or `discordapp.com` under the path
`/api/webhooks/`. An unknown type or an invalid setting SHALL be rejected
with a validation error naming the field, saving nothing. Deleting a
channel SHALL delete its rules. The edge box SHALL NOT serve these
endpoints.

#### Scenario: Adding a Discord channel

- **WHEN** an admin posts `{"name": "Domicilios", "type": "discord", "settings": {"webhook_url": "https://discord.com/api/webhooks/123/abc"}}`
- **THEN** the response is `201` with the channel, `enabled: true`

#### Scenario: A webhook outside Discord

- **WHEN** an admin posts a `discord` channel with `webhook_url` `https://example.com/hook`
- **THEN** the request fails with a validation error naming `settings.webhook_url` and no channel is created

#### Scenario: Another organization's channel

- **WHEN** a manager of organization B requests a channel of organization A by id
- **THEN** the response is `404`

#### Scenario: A manager reads but cannot change

- **WHEN** a manager lists channels, then posts, puts or deletes a channel
- **THEN** the list is `200`, and each change is `403` with nothing changed

#### Scenario: Without the permission

- **WHEN** a user without `notification-channels:manage` posts a channel
- **THEN** the response is `403` and nothing is created

### Requirement: Channel secrets are encrypted and never returned

A channel's secret settings SHALL be stored encrypted with a key the
deployment provides, so the stored value alone does not reveal them. No
response SHALL contain a secret. In their place a channel SHALL show
`settings.webhook_url_hint`, the last four characters of the URL after
`…`. On `PUT`, an omitted secret SHALL keep the stored one. When the
deployment has no key, the notification endpoints SHALL respond `503` with
the code `NOTIFICATIONS_DISABLED`, and no notification SHALL be recorded or
sent.

#### Scenario: Reading a channel

- **WHEN** a manager gets a channel whose webhook ends in `.../abcd1234`
- **THEN** the response has `settings.webhook_url_hint` `…1234` and no `webhook_url`

#### Scenario: Renaming keeps the webhook

- **WHEN** an admin puts `{"name": "Cocina"}` on a channel without `settings`
- **THEN** the name changes and the channel still delivers to the same webhook

#### Scenario: No key configured

- **WHEN** the deployment has no notification key and a manager lists channels
- **THEN** the response is `503` with the code `NOTIFICATIONS_DISABLED`

### Requirement: Staff can send a test message to a channel

`POST /api/notification-channels/:id/test` SHALL send a sample message to
the channel right away, requiring `notification-channels:manage`, and
SHALL respond `204` when the destination accepts it. When the destination
refuses it or cannot be reached, the response SHALL be `502` with the code
`NOTIFICATION_CHANNEL_UNREACHABLE` and the status the destination
returned, if any. A test SHALL NOT be stored or retried.

#### Scenario: A working webhook

- **WHEN** an admin tests a channel whose webhook exists
- **THEN** the response is `204` and the sample message appears in the Discord channel

#### Scenario: A deleted webhook

- **WHEN** an admin tests a channel whose webhook Discord answers with `404`
- **THEN** the response is `502` with the code `NOTIFICATION_CHANNEL_UNREACHABLE` and `destination_status` `404`

#### Scenario: A manager cannot test

- **WHEN** a manager tests a channel
- **THEN** the response is `403` and nothing is sent

### Requirement: Staff manage notification rules

The cloud SHALL serve `GET`, `POST /api/notification-rules` and `GET`,
`PUT`, `DELETE /api/notification-rules/:id`, with the same permissions and
organization scoping as channels: admins and managers read, only admins
change. A rule SHALL have:

- `channel_id`: a channel of the same organization
- `event`: one of the catalogued events
- `conditions`: only the conditions of its event
- `title_prefix`: optional, up to 100 characters
- `expires_after_minutes`: 1–10080. It defaults to 60 for order events and
  720 for reservation events.
- `enabled`: default `true`

An unknown event, a condition its event does not have, an unknown value in
a condition, or a channel of another organization SHALL be rejected with a
validation error naming the field, saving nothing. `GET` SHALL accept a
`channel_id` filter.

#### Scenario: Paid deliveries to the delivery channel

- **WHEN** an admin posts a rule with `event` `order.paid`, `conditions` `{"order_types": ["delivery"]}` and the delivery channel
- **THEN** the response is `201` with `expires_after_minutes` 60 and `enabled: true`

#### Scenario: A condition the event does not have

- **WHEN** an admin posts a rule with `event` `order.paid` and `conditions` `{"min_party_size": 6}`
- **THEN** the request fails with a validation error naming `conditions.min_party_size`

#### Scenario: Another organization's channel

- **WHEN** an admin of organization B posts a rule with a channel of organization A
- **THEN** the request fails with a validation error naming `channel_id`

#### Scenario: A manager cannot change a rule

- **WHEN** a manager posts, puts or deletes a rule
- **THEN** the response is `403` and nothing changes

## ADDED Requirements

### Requirement: Ajustes is a hub of settings sections

The ops UI SHALL serve `/ajustes` as a hub with one card per settings
section, each shown only to users holding that section's read permission:
Integraciones (`integrations:read`), linking to `/ajustes/integraciones`,
and Notificaciones (`notification-channels:read`), linking to
`/ajustes/notificaciones`. The hub and its home navigation entry SHALL be
shown to users holding at least one of these permissions. A section page
opened without its permission SHALL show a no-access message instead of
its content. The Integraciones page SHALL keep the behavior the Wompi card
had on `/ajustes`. On an edge box the notification permissions SHALL be
dropped like the integration ones, so neither section appears.

#### Scenario: An admin sees both sections

- **WHEN** an admin opens `/ajustes` in the cloud
- **THEN** the hub shows the Integraciones and Notificaciones cards

#### Scenario: A manager sees only notifications

- **WHEN** a manager opens the home screen and then `/ajustes` in the cloud
- **THEN** Ajustes is in the navigation, and the hub shows only the Notificaciones card

#### Scenario: A manager opens integrations by URL

- **WHEN** a manager opens `/ajustes/integraciones`
- **THEN** the page shows a no-access message and no Wompi state

#### Scenario: Hidden on the edge

- **WHEN** an admin or a manager signs in on an edge box
- **THEN** Ajustes is not in the navigation and `/ajustes/notificaciones` shows no channels

### Requirement: The Notificaciones page shows channels with their rules

`/ajustes/notificaciones` SHALL list the organization's channels. Each one
SHALL show its name, type, enabled state and webhook hint, with its rules
nested under it. Each rule SHALL show its event, its conditions, its
expiry and its enabled state, all with Spanish labels:

- events: `order.created` "Pedido creado", `order.paid` "Pedido pagado",
  `reservation.requested` "Reserva por confirmar", `reservation.confirmed`
  "Reserva confirmada"
- `order_types`: "En mesa", "Para llevar", "Domicilio"
- `sources`: "WhatsApp", "Teléfono", "Sin reserva", "Personal"
- `confirmed_by`: "Por el personal", "Automáticamente"
- `min_party_size`: "Mínimo de personas"

The page SHALL NOT display a webhook URL, only its hint. When the server
answers `503 NOTIFICATIONS_DISABLED`, the page SHALL say that
notifications are not enabled on this server and SHALL offer no form.

#### Scenario: A channel with two rules

- **WHEN** an admin opens the page with a "Domicilios" channel that has an `order.paid` rule for `delivery` and a `reservation.requested` rule for `whatsapp` with `min_party_size` 6
- **THEN** "Domicilios" shows `••••1234` and, under it, "Pedido pagado · Domicilio" and "Reserva por confirmar · WhatsApp · Mínimo de personas: 6"

#### Scenario: No channels yet

- **WHEN** an admin opens the page and the organization has no channels
- **THEN** the page says there are no channels and offers to add one

#### Scenario: Server without a notification key

- **WHEN** the cloud runs without `NOTIFICATION_SECRETS_KEY`
- **THEN** the page says notifications are not enabled on this server and offers no form

### Requirement: Admins manage channels and rules from the Notificaciones page

A user with `notification-channels:manage` SHALL be able to add a Discord
channel, rename it, enable or disable it, replace its webhook, delete it
after confirming in the page, and send it a test message. They SHALL be
able to add, edit, enable or disable and delete a rule under a channel.
The rule form SHALL offer only the conditions of the chosen event, SHALL
leave the expiry empty to take the server's default, and SHALL lock the
event when editing. The form SHALL have an optional role ID field. When it
is filled, the UI SHALL send `title_prefix` as `<@&ID>` followed by the
free-text prefix, if any, separated by a space. When editing a rule whose
`title_prefix` starts with `<@&digits>`, the form SHALL show the ID in the
role field and the rest in the free-text field. The webhook input SHALL be
empty after saving. A refused save SHALL show its reason next to the field
the server names, or as the form's message when none is named.

#### Scenario: Adding a channel and testing it

- **WHEN** an admin adds "Domicilios" with a Discord webhook and chooses Probar
- **THEN** the channel appears with its hint, the webhook input is empty, and the page says the test message was sent

#### Scenario: A test Discord refuses

- **WHEN** an admin tests a channel whose webhook Discord answers with `404`
- **THEN** the page says Discord did not accept the message and shows `404`

#### Scenario: A webhook outside Discord

- **WHEN** an admin saves a channel with `https://example.com/hook`
- **THEN** the error is shown next to the webhook input and no channel is added

#### Scenario: Conditions follow the event

- **WHEN** an admin creating a rule picks "Pedido pagado", then switches to "Reserva confirmada"
- **THEN** the form first offers only the order types, then the sources, "Confirmada por" and the minimum party size

#### Scenario: Mentioning a role

- **WHEN** an admin saves a rule with role ID `123456789` and free-text prefix `Atención`
- **THEN** the rule is sent with `title_prefix` `<@&123456789> Atención`, and reopening it shows `123456789` and `Atención` in their fields

#### Scenario: A role ID with letters

- **WHEN** an admin types `abc` in the role ID field
- **THEN** the form shows that the ID must be digits only and does not send the rule

#### Scenario: Deleting a channel

- **WHEN** an admin deletes a channel with two rules and confirms in the page
- **THEN** the channel and its rules disappear from the page

### Requirement: Managers see notifications read-only

A user holding `notification-channels:read` without
`notification-channels:manage` SHALL see the same channels and rules
without any add, edit, delete, enable, disable or test control, and SHALL
see a notice that only an admin can change notifications.

#### Scenario: A manager opens the page

- **WHEN** a manager opens `/ajustes/notificaciones` with one channel and one rule
- **THEN** both are shown, with no Probar button and no forms or switches, and a notice says only an admin can change them
