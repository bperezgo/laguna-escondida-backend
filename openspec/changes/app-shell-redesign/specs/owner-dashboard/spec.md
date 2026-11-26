## Purpose

Gives the person who runs the restaurant a first screen in the cloud that shows how the day is going, in place of a menu of modules, and says what to expect when there are no sales yet.

## ADDED Requirements

### Requirement: The dashboard is the cloud's first screen for finance readers

In the cloud, a user who may read finance figures SHALL get the dashboard as their first screen and as the Inicio entry of the navigation. A user who may not SHALL NOT be shown the dashboard or any of its figures, and opening its address SHALL take them to their own first screen. The box SHALL have no dashboard.

#### Scenario: An admin

- **WHEN** an admin opens Inicio in the cloud
- **THEN** the dashboard is shown

#### Scenario: A user without finance access

- **WHEN** a manager who may not read finance figures opens the dashboard's address
- **THEN** their own first screen opens and no figure is shown

#### Scenario: The box

- **WHEN** an admin opens the dashboard's address on the box
- **THEN** their first screen on the box opens

### Requirement: The dashboard shows the current business day

The dashboard SHALL show, for the current business day in the restaurant's time zone:

- the amount collected;
- the number of paid orders;
- the average ticket, which is the amount collected divided by the number of paid orders;
- the amount collected by each payment method;
- the expenses recorded;
- the purchases recorded.

Amounts SHALL be shown in the organization's currency and number format. The dashboard SHALL name the day the figures belong to. It SHALL link to the financial summary and to the daily close.

#### Scenario: A day with sales

- **WHEN** the day has 12 paid orders that collected $1.920.000, of which $1.240.000 in cash and $680.000 by card
- **THEN** the dashboard shows $1.920.000 collected, 12 orders, an average ticket of $160.000, and the two payment methods with their amounts

#### Scenario: Expenses and purchases

- **WHEN** the day has $150.000 of expenses and $300.000 of purchases recorded
- **THEN** the dashboard shows both amounts

#### Scenario: The day shown

- **WHEN** the dashboard is opened at 11 p.m. Bogotá time on 25 November
- **THEN** it says the figures are for 25 November

### Requirement: A day without sales explains itself

When the day has no paid orders, the dashboard SHALL replace the sales figures with a message saying that the day's sales appear there as orders are paid on the box. It SHALL NOT show an average ticket. It SHALL NOT show example or invented figures. For a user who may install a box, the message SHALL link to the box installation screen. Expenses and purchases of the day SHALL still be shown.

#### Scenario: A new account

- **WHEN** an admin of an organization with no orders opens the dashboard
- **THEN** the message is shown in place of the sales figures, no average ticket is shown, and no amount other than zero-valued expenses and purchases appears

#### Scenario: The link to install the box

- **WHEN** an admin, who may install a box, sees that message
- **THEN** it links to the box installation screen

### Requirement: A failure to load does not block the app

When the figures cannot be loaded, the dashboard SHALL say so, SHALL offer to try again, and SHALL leave the navigation usable. It SHALL NOT show figures from an earlier day as if they were current. While the figures load, the dashboard SHALL show that it is loading without moving the navigation.

#### Scenario: The request fails

- **WHEN** the figures cannot be loaded
- **THEN** a message says so with an action to try again, and the navigation still opens other modules

#### Scenario: Trying again

- **WHEN** the user chooses to try again and the figures load
- **THEN** the figures replace the message
