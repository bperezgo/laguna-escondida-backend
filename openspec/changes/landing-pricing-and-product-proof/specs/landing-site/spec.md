## MODIFIED Requirements

### Requirement: The home page presents the product in Spanish

The site SHALL serve a home page in Colombian Spanish that presents, each in its own section: selling while the internet is down; delivery and takeaway; electronic invoicing for DIAN; the kitchen screen and back office (stock, purchases, expenses, daily close); the WhatsApp agent, as an add-on (see "The WhatsApp agent is presented as an add-on"); and the plans and their prices (see "The pricing section"). It SHALL include a section answering common objections as questions and answers. Product images SHALL be screenshots of the real product showing demo data only. The page SHALL NOT show invented testimonials, customer logos or figures. A price SHALL appear only when the owner supplied it, and SHALL NOT appear for anything the owner has not priced.

#### Scenario: A visitor reads the page

- **WHEN** a visitor opens the home page
- **THEN** each of the product areas, the plans section and the questions section is present, in Spanish

#### Scenario: Screenshots carry no real guest data

- **WHEN** the page's product images are reviewed
- **THEN** every name, phone and address shown belongs to demo data

#### Scenario: The question about cost

- **WHEN** a visitor opens the question about what the product costs
- **THEN** the answer gives the plans' prices or points to the plans section, and does not say that prices are not published

### Requirement: One primary action, always in reach

The page SHALL have one primary action and one secondary action:

- When the signup address is configured, the primary action SHALL be starting the 14-day trial, which leads to the app's signup page, and the secondary action SHALL be requesting a demo, which leads to the lead form.
- When the signup address is not configured, the primary action SHALL be requesting a demo and no trial action SHALL be shown.

The primary action SHALL be visible without scrolling at a viewport 360 pixels wide, SHALL be repeated at the end of the page, and SHALL stay reachable while scrolling on a phone. Every primary action on the site SHALL use the same label, and every secondary action its own same label. Only the primary action SHALL use the filled button style.

#### Scenario: First screen on a phone

- **WHEN** the home page loads at 360 by 640 pixels
- **THEN** the headline and the primary action are visible without scrolling

#### Scenario: Signup is open

- **WHEN** the site is built with a signup address
- **THEN** the primary action everywhere is "Prueba 14 días" and leads to that address, and "Pide una demo" is offered as the secondary action leading to the lead form

#### Scenario: Signup is not open

- **WHEN** the site is built without a signup address
- **THEN** the primary action everywhere is "Pide una demo", and no trial action or sign-in link is shown

### Requirement: Conversion and visit events

The site SHALL report to Google Analytics and to the Meta Pixel:
- a page view on every page load;
- on a successful lead submission, and only then, `generate_lead` to Google Analytics and `Lead` to Meta, both carrying the same `event_id` that was sent with the lead;
- on activating the WhatsApp action, `contact_whatsapp` to Google Analytics and `Contact` to Meta;
- on activating the trial action, `trial_click` to Google Analytics and the custom event `TrialClick` to Meta.

A failed or rejected submission SHALL NOT report a lead event. No event SHALL carry the visitor's name, restaurant name or phone. Reporting the trial event SHALL NOT delay or prevent the navigation to the signup page.

#### Scenario: A lead converts

- **WHEN** a visitor's form submission is answered `201`
- **THEN** one `generate_lead` and one `Lead` event are reported, with the `event_id` sent to the backend, and neither contains the name or phone

#### Scenario: A submission fails

- **WHEN** a visitor's form submission is answered `429`
- **THEN** no `generate_lead` or `Lead` event is reported

#### Scenario: A visitor starts the trial

- **WHEN** a visitor activates any trial action on the site
- **THEN** one `trial_click` and one `TrialClick` event are reported, and the signup page opens

#### Scenario: The trial action with tracking blocked

- **WHEN** both tracking scripts are blocked and a visitor activates the trial action
- **THEN** the signup page opens

## ADDED Requirements

### Requirement: The pricing section

The home page SHALL show the three plans with their names, their monthly price in Colombian pesos and what each includes:

| Plan | Monthly price |
|---|---|
| Básico | $25.000 |
| Facturación electrónica | $40.000 |
| Completo | $100.000 |

It SHALL say that prices are before IVA. It SHALL offer annual payment at ten times the monthly price, described as two months free, and SHALL show that annual price for each plan. It SHALL say that the system is installed on the computer the restaurant already has. Each plan SHALL carry the primary action. The section SHALL NOT offer a way to pay on the site.

#### Scenario: A visitor compares the plans

- **WHEN** a visitor opens the plans section
- **THEN** the three plans are shown with $25.000, $40.000 and $100.000 per month, what each includes, and a note that prices are before IVA

#### Scenario: Annual payment

- **WHEN** a visitor reads the Completo plan
- **THEN** its annual price is shown as $1.000.000, with the two free months stated

#### Scenario: No payment on the site

- **WHEN** a visitor activates a plan's action
- **THEN** the primary action's destination opens, and no payment detail is asked for on the site

### Requirement: The header navigation

Every page SHALL show a header with the product name linking to the home page, links to the home page's sections for selling without internet, the WhatsApp agent, the plans and the questions, and the primary action. When the signup address is configured, the header SHALL also link to the app's sign-in page. At viewports below 768 pixels the section links and the sign-in link SHALL be reachable from a control in the header that works without scripts, and the primary action SHALL stay visible in the header. On a page other than the home page, a section link SHALL lead to that section of the home page.

#### Scenario: A visitor goes to the plans

- **WHEN** a visitor on the home page activates the plans link in the header
- **THEN** the page moves to the plans section

#### Scenario: From another page

- **WHEN** a visitor on the download page activates the plans link in the header
- **THEN** the home page opens at the plans section

#### Scenario: A phone

- **WHEN** the home page is open at 360 pixels wide
- **THEN** the header shows the product name, the primary action and a control that reveals the links, with no horizontal scrolling

#### Scenario: An existing customer

- **WHEN** the site is built with a signup address and a visitor activates the sign-in link
- **THEN** the app's sign-in page opens

### Requirement: The WhatsApp agent is presented as an add-on

The home page SHALL present the WhatsApp agent, which takes orders and reservations for the restaurant, as a separate product that connects to the system and is contracted apart. Its section SHALL say so visibly. It SHALL NOT show a price for the agent. Its action SHALL be asking about the agent: the WhatsApp action with a message about the agent when a number is configured, and the lead form otherwise. Nowhere on the site, including the plans, the steps to start, the questions and the page's title and description, SHALL the agent or a WhatsApp channel be described as included in a plan. The plan that lets an agent connect SHALL say that it allows the connection and that the agent is contracted apart.

#### Scenario: A visitor reads the agent's section

- **WHEN** a visitor opens the WhatsApp agent's section
- **THEN** it is labelled as an add-on contracted apart, shows no price, and offers to ask about it

#### Scenario: The plans

- **WHEN** a visitor reads what the Completo plan includes
- **THEN** it says the plan allows connecting the WhatsApp agent and that the agent is contracted apart

#### Scenario: The steps to start

- **WHEN** a visitor reads the steps to start and the questions
- **THEN** none of them says a WhatsApp channel is set up as part of starting

### Requirement: The outage and the WhatsApp order are shown as sequences

The section on selling without internet SHALL show, as three numbered steps each with a product image: the box working with no connection; an order being taken or paid during the outage; and that work synchronised once the connection returns. The WhatsApp agent's section SHALL show, as three numbered steps, a customer's message asking for an order, that order in the system, and its ticket in the kitchen once the order is paid; the last two SHALL be product images. The customer's message MAY be drawn instead of captured, and SHALL then be marked as an example. Each sequence SHALL be readable in order with scripts off and with reduced motion.

#### Scenario: The outage sequence

- **WHEN** a visitor reads the section on selling without internet
- **THEN** three numbered steps are shown in order, each with an image of the product in that state

#### Scenario: The WhatsApp sequence

- **WHEN** a visitor reads the WhatsApp agent's section
- **THEN** three numbered steps go from the customer's message to the order in the system to the kitchen ticket, and the message is marked as an example

#### Scenario: Scripts are off

- **WHEN** the home page is opened with scripts disabled
- **THEN** every step of both sequences and its image is visible

### Requirement: A missing product image fails the build

Every product image the site refers to SHALL exist when the site is built. A build in which one is missing SHALL fail and name the missing image. A page SHALL never be published with an empty place where a product image belongs.

#### Scenario: An image is missing

- **WHEN** the site is built and the image for the kitchen section is absent
- **THEN** the build fails with a message naming that image

#### Scenario: Every image is present

- **WHEN** the site is built with every referred image present
- **THEN** each image is in the built page with its description
