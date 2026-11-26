## Purpose

Presents the product to restaurant owners who arrive from paid ads, turns a visit into a demo request, and reports that conversion to Google Analytics and Meta so ad spend can be measured and optimised.

## ADDED Requirements

### Requirement: The home page presents the product in Spanish

The site SHALL serve a home page in Colombian Spanish that presents, each in its own section: selling while the internet is down; orders and reservations taken over WhatsApp; delivery and takeaway; electronic invoicing for DIAN; and the kitchen screen and back office (stock, purchases, expenses, daily close). It SHALL include a section answering common objections as questions and answers. Product images SHALL be screenshots of the real product showing demo data only. The page SHALL NOT show invented testimonials, customer logos, prices or figures.

#### Scenario: A visitor reads the page

- **WHEN** a visitor opens the home page
- **THEN** each of the five product areas and the questions section is present, in Spanish

#### Scenario: Screenshots carry no real guest data

- **WHEN** the page's product images are reviewed
- **THEN** every name, phone and address shown belongs to demo data

### Requirement: One primary action, always in reach

The page SHALL have one primary action, requesting a demo, which leads to the lead form. It SHALL be visible without scrolling at a viewport 360 pixels wide, SHALL be repeated at the end of the page, and SHALL stay reachable while scrolling on a phone. Every primary action on the page SHALL use the same label.

#### Scenario: First screen on a phone

- **WHEN** the home page loads at 360 by 640 pixels
- **THEN** the headline and the primary action are visible without scrolling

### Requirement: The lead form

The form SHALL ask for a name, the restaurant's name and a WhatsApp number, and SHALL carry a consent checkbox, unchecked by default, with a link to the privacy policy. It SHALL NOT submit until all three fields are filled and consent is given, and SHALL say which field is missing or invalid. While a submission is in flight the submit control SHALL be disabled. On success the form SHALL be replaced by a confirmation, without leaving the page. On a rejected field it SHALL show the error next to that field. On any other failure it SHALL keep what the visitor typed, say that sending failed, and offer the WhatsApp action.

#### Scenario: A successful request

- **WHEN** a visitor fills the three fields, gives consent and submits, and the backend answers `201`
- **THEN** the form is replaced by a confirmation that they will be contacted on WhatsApp

#### Scenario: Consent not given

- **WHEN** a visitor fills the three fields and submits without checking consent
- **THEN** nothing is sent and the form asks for consent

#### Scenario: The backend is unreachable

- **WHEN** a visitor submits a valid form and the request fails
- **THEN** their input is still in the form, an error is shown, and the WhatsApp action is offered

#### Scenario: A double click

- **WHEN** a visitor clicks submit twice quickly
- **THEN** one request is sent

### Requirement: The WhatsApp action

The page SHALL offer, as a secondary action, a link that opens a WhatsApp chat with the configured number and a prefilled Spanish message. When no number is configured, the action SHALL NOT be shown anywhere on the site.

#### Scenario: A visitor prefers to chat

- **WHEN** a visitor activates the WhatsApp action
- **THEN** WhatsApp opens on a chat with the configured number and the prefilled message

### Requirement: Attribution is captured on arrival and sent with the lead

On a visitor's first page of a session, the site SHALL record the `utm_source`, `utm_medium`, `utm_campaign`, `utm_term`, `utm_content`, `fbclid` and `gclid` query parameters that are present, the landing URL and the referrer. These SHALL be kept for the session, SHALL NOT be overwritten by later pages of the same session, and SHALL be sent with the lead. When the browser refuses storage, the form SHALL still submit, with whatever attribution the current page has.

#### Scenario: Arriving from an ad, reading the policy, then converting

- **WHEN** a visitor lands with `?utm_source=facebook&utm_campaign=lanzamiento&fbclid=abc`, opens the privacy policy, returns and submits the form
- **THEN** the lead is sent with `utm_source` `facebook`, `utm_campaign` `lanzamiento`, `fbclid` `abc` and the original landing URL

#### Scenario: Storage is blocked

- **WHEN** a visitor in a browser that blocks storage submits the form
- **THEN** the lead is sent

### Requirement: Conversion and visit events

The site SHALL report to Google Analytics and to the Meta Pixel:
- a page view on every page load;
- on a successful lead submission, and only then, `generate_lead` to Google Analytics and `Lead` to Meta, both carrying the same `event_id` that was sent with the lead;
- on activating the WhatsApp action, `contact_whatsapp` to Google Analytics and `Contact` to Meta.

A failed or rejected submission SHALL NOT report a lead event. No event SHALL carry the visitor's name, restaurant name or phone.

#### Scenario: A lead converts

- **WHEN** a visitor's form submission is answered `201`
- **THEN** one `generate_lead` and one `Lead` event are reported, with the `event_id` sent to the backend, and neither contains the name or phone

#### Scenario: A submission fails

- **WHEN** a visitor's form submission is answered `429`
- **THEN** no `generate_lead` or `Lead` event is reported

### Requirement: Tracking is configured, optional and never in the way

The Google Analytics measurement id, the Meta Pixel id, the WhatsApp number, the backend address, the product name and the site address SHALL be configuration, not text repeated across pages. When a tracking id is not configured, that tool SHALL NOT be loaded and the site SHALL work the same otherwise. Tracking scripts SHALL NOT delay the page's first render, and a tracking script that fails to load or is blocked SHALL NOT break the form or any other part of the page.

#### Scenario: A visitor with an ad blocker

- **WHEN** both tracking scripts are blocked and the visitor submits a valid form
- **THEN** the lead is sent and the confirmation is shown

#### Scenario: No pixel id configured

- **WHEN** the site is built without a Meta Pixel id
- **THEN** no page requests the Meta Pixel script, and Google Analytics events are still reported

### Requirement: The privacy policy

The site SHALL serve a privacy policy page in Spanish, linked from the form's consent line and from every page's footer. It SHALL state who is responsible for the data, what the form collects and why, that Google Analytics and the Meta Pixel are used, and how to ask for one's data to be corrected or deleted.

#### Scenario: A visitor checks the policy before submitting

- **WHEN** a visitor follows the link in the consent line
- **THEN** the policy page opens and covers those four points

### Requirement: The site can be found and shared

Each page SHALL have a title, a description, a canonical address and social sharing metadata with an image. The site SHALL serve a sitemap and a robots file that allow indexing.

#### Scenario: The link is shared in WhatsApp

- **WHEN** the home page's address is pasted into a chat
- **THEN** the preview shows the product name, a description and an image
