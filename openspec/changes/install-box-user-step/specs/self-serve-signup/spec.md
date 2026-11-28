## RENAMED Requirements

- FROM: `### Requirement: Signup creates the organization, its admin and its box user`
- TO: `### Requirement: Signup creates the organization and its admin`

## MODIFIED Requirements

### Requirement: Signup creates the organization and its admin

Creating an account from a signup SHALL store, together or not at all, the organization and the admin described below. It SHALL NOT create any other user: the box's users are created by the admin.

**The organization:**
- `name` and `display_name` are the restaurant name.
- Its `slug` is derived from the restaurant name: lowercased, accents removed, non-alphanumeric runs replaced by `-`, cut to 40 characters, and with `-2`, `-3`, … appended when already taken.
- `status` is `trial` and `trial_ends_at` is 14 days after creation.
- `plan` is the chosen plan.
- `timezone`, `country`, `locale` and `currency` are `America/Bogota`, `CO`, `es-CO` and `COP`.
- Business info, receipt text, branding and e-invoicing prefixes are unset.

**The admin:** the Google account's name and email, with the admin role and no password; they sign in with Google. Its username is its email.

#### Scenario: Organization created from a signup

- **WHEN** "Restaurante Doña Rosa" signs up on plan `einvoicing` with `rosa.perez@example.com`
- **THEN** the organization has slug `restaurante-dona-rosa`, status `trial`, plan `einvoicing` and `trial_ends_at` 14 days ahead
- **AND** its admin has username `rosa.perez@example.com`
- **AND** the admin is the organization's only user

#### Scenario: Slug taken

- **WHEN** a second restaurant named "Restaurante Doña Rosa" signs up
- **THEN** its slug is `restaurante-dona-rosa-2`

#### Scenario: Short email name

- **WHEN** the email is `jo@example.com`
- **THEN** the admin's username is `jo@example.com`
- **AND** no user named `caja` or `jo` is created

## REMOVED Requirements

### Requirement: An admin generates a new password for a user without an email

**Reason**: The admin now chooses the box user's password, so the cloud no longer generates one. A generated password went to a hidden user the admin did not know about.

**Migration**: Set a box user's password with `POST /api/admin/users/:id/reset-password` (Usuarios → Contraseña, or "Cambiar contraseña" in Instalar caja). `POST /api/admin/users/:id/generate-password` answers `404`.
