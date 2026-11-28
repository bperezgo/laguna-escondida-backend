## ADDED Requirements

### Requirement: The wizard shows that it is working while it waits

When the owner presses Next on a wizard screen that then waits on a check of the PC, a check of the network, or a call to the cloud, the wizard SHALL show a working screen, with a moving bar and a message naming what it is doing, until the wait ends. It SHALL then continue to the next screen, or return to the screen the owner was on with the same message it shows today. A silent install SHALL show no working screen.

#### Scenario: Waiting for the cloud after the code

- **WHEN** the owner types the code and presses Next on the sign-in screen
- **THEN** a working screen reading `Conectando con Kuenta…` is shown until the cloud answers
- **AND** then the wizard continues to the installation

#### Scenario: Checking the PC from the welcome screen

- **WHEN** the owner presses Next on the welcome screen
- **THEN** a working screen reading `Revisando el equipo…`, then `Revisando la red…`, is shown until the checks end

#### Scenario: A failed check returns to the same screen

- **WHEN** the code is not valid and the cloud refuses it
- **THEN** the working screen closes, the sign-in screen is shown again, and the same error message as today is shown

#### Scenario: Replacing a box still asks first

- **WHEN** the organization already has a box
- **THEN** the working screen closes before the replace question is shown
- **AND** after the owner answers yes, the working screen is shown again for the second call

#### Scenario: Silent install is unchanged

- **WHEN** the installer runs with `/VERYSILENT`
- **THEN** no working screen is shown, and the install behaves as before

### Requirement: A press during a wait does nothing

While the working screen is shown, the wizard's Next, Back and Cancel buttons SHALL NOT act, so a second press SHALL NOT start a second check or a second enrollment.

#### Scenario: A second press during the wait does nothing

- **GIVEN** the working screen is shown after the owner pressed Next on the sign-in screen
- **WHEN** the owner presses Next, Enter, or anywhere on the window again before the cloud answers
- **THEN** the cloud receives exactly one enrollment call for that press
- **AND** the wizard shows the same next screen as with a single press
