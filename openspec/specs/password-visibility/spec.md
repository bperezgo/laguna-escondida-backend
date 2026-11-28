# password-visibility Specification

## Purpose
Lets a person show and hide what they typed in a password field (sign-in, invitation, create-user and reset-password forms) without exposing it longer than they chose: hidden by default and hidden again on submit.

## Requirements

### Requirement: A password field can show what was typed

The password field on the sign-in page (cloud and box), on the invitation page, and in the create-user and reset-password forms SHALL have a toggle button inside the field, shown as an eye icon.
- The field SHALL start hidden (`type="password"`) every time the form is shown.
- Pressing the toggle SHALL show the typed text; pressing it again SHALL hide it. The value and the caret position SHALL be kept, and focus SHALL return to the field.
- The toggle SHALL be a `type="button"`: pressing it SHALL NOT submit the form, and pressing Enter in the field SHALL still submit it.
- The toggle SHALL be reachable with Tab and operable with Enter and Space. Its accessible name SHALL be "Mostrar contraseña" while the text is hidden and "Ocultar contraseña" while it is shown, and it SHALL carry `aria-pressed` matching the state.
- Its touch target SHALL be at least 44×44 CSS px, so it works on the box's touch screen.
- Where a form has a confirm field, one toggle SHALL show or hide both.
- The toggle SHALL be disabled while the field is disabled.

#### Scenario: Show and hide on sign-in

- **WHEN** a person types a password on the sign-in page and presses the eye button
- **THEN** the password is shown as text, the button's name is "Ocultar contraseña" with `aria-pressed="true"`, and the typed value is unchanged
- **AND WHEN** they press it again
- **THEN** the password is hidden and the button's name is "Mostrar contraseña" with `aria-pressed="false"`

#### Scenario: The toggle does not submit

- **WHEN** a person presses the eye button with the keyboard (Enter or Space) or by touch
- **THEN** the form is not submitted and no sign-in request is sent

#### Scenario: Keyboard only

- **WHEN** a person tabs from the password field
- **THEN** focus moves to the eye button, and Space toggles it

### Requirement: A shown password does not stay on screen

A shown password SHALL go back to hidden when the form is submitted, whether the submit succeeds or fails, and SHALL be hidden whenever the form is shown again. Showing the password SHALL NOT change the field's `name`, `autoComplete` or value, and the password SHALL NOT be written to logs, storage or the URL by this feature.

#### Scenario: Hidden again after a failed sign-in

- **WHEN** a person shows the password and submits a wrong one
- **THEN** after the error appears the field is hidden again (`type="password"`)

#### Scenario: Hidden by default after navigating back

- **WHEN** a person shows the password, leaves the sign-in page and comes back
- **THEN** the field starts hidden

#### Scenario: Browser password managers still work

- **WHEN** the sign-in form is submitted with the password shown
- **THEN** the field's `name` is still `password` and the request body is the same as with the password hidden
