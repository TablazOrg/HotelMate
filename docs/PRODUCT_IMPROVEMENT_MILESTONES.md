# Product improvement milestones: M8–M12

## Purpose

This roadmap improves HotelMate from a secure functional MVP into a competitive hotel guest-experience platform. It is based on a source-code/API review, hands-on review of the local guest and staff interfaces, and current official product material from Duve, Canary, and HiJiffy.

Competitor material is vendor-published and should be treated as evidence of category expectations, not independent proof of business impact. HotelMate should validate every proposed feature with target hotels and guests before committing to a provider or workflow.

## Current HotelMate baseline

HotelMate already has valuable foundations:

- A branded, no-download Persian RTL web experience for public visitors, pre-arrival guests, active guests, staff departments, and administrators.
- Tenant-bound reservation login, pre-arrival stays, secure identity-document upload/review, check-in/check-out, and room-state transitions.
- Six quick requests, paid and pre-arrival services, live request tracking, role-filtered fulfillment queues, and persisted request history.
- Public facilities, promotions, restaurant/menu content, an approved-knowledge assistant, reception handoff, reporting, and audit administration.
- PostgreSQL integration tests, explicit migrations, private document retention, OpenAPI, Docker release paths, smoke checks, and end-to-end acceptance.

The local UX and code review also exposed product gaps:

- Guest entry requires a hotel slug plus reservation/room identifiers; there are no secure invitation links, magic links, phone verification, or on-property QR recovery.
- “Online check-in” currently uploads one PDF/JPEG/PNG and supports staff approval/rejection. Guest details, companions, arrival time, transport, custom questions, preferences, legal consent, signature, payment/deposit, and room-ready access are not part of the workflow.
- The pre-arrival timeline explicitly shows arrival time as a future step, and staff see document status rather than overall arrival readiness and exceptions.
- Service requests are fast but one-dimensional: no service detail, quantity/modifiers, delivery slot, basket, availability inventory, structured confirmation, or receipt.
- Request tracking mixes active and historic items in a long feed; repeated requests are hard to distinguish and there is no itinerary for scheduled services.
- Chat is in-app only. There is no outbound lifecycle automation, delivery/fallback state, WhatsApp/SMS/email adapter, opt-out workflow, or unified cross-channel inbox.
- Content and upsells are hotel-scoped but not personalized by stay stage, guest preference, inventory, room type, party, or prior behavior.
- Payments remain “pay at hotel”; there is no PSP boundary, deposit/hold, payment link, refund, reconciliation, folio, or digital checkout.
- There is no PMS/CRS, POS, CRM, channel-manager, or digital-key connector contract.
- Operational reports measure outcomes but not guest-journey funnels, abandonment, time-on-step, message delivery, SLA aging, or offer conversion.
- The UI is responsive and visually coherent, but accessibility, multilingual operation, real-user usability, performance budgets, offline recovery, and design-system consistency have not been independently verified.

## Competitive benchmark

| Capability | HotelMate today | Category benchmark | Product direction |
| --- | --- | --- | --- |
| One guest surface | Branded web UI, but guests manually enter hotel and reservation/room identifiers | Duve presents check-in, guest information, messaging, ordering, upsells, and checkout in one no-download branded link. [Duve Guest App](https://duve.com/hotel-guest-app/) | Preserve the web-first model while moving to secure reservation links, QR recovery, and one lifecycle-aware home |
| Online check-in depth | Reservation lookup plus one identity document and staff review | Duve's wizard includes configurable details, transportation, questions, signatures, per-guest documents, stay payment, deposits, and upsells. [Duve check-in wizard](https://helpcenter.duve.com/hc/en-us/articles/26934895654941-Online-Check-In-Wizard-Latest-Updates-Overview) | Build a resumable configurable arrival workflow and an exception-based staff review queue |
| Identity and registration | MIME validation, private storage, checksum, retention, manual approve/reject | Duve supports required documents per guest, retention choices, e-signature, and optional MRZ verification. [Required documents](https://helpcenter.duve.com/hc/en-us/articles/7636458126109-Required-Documents), [E-signature](https://helpcenter.duve.com/hc/en-us/articles/7716772163613-E-Signature), [MRZ verification](https://helpcenter.duve.com/hc/en-us/articles/24554539873309-MRZ-Verification-for-Required-Documents-in-the-Online-Check-in) | Add configurable per-guest evidence, versioned consent/signature, pluggable verification, and strict regional retention controls |
| Contactless arrival | Staff completes check-in and room allocation | Duve documents QR-triggered identification, online check-in, wait mode, and digital-key access; Canary combines mobile check-in, tablet/kiosk options, and mobile keys. [Duve contactless flow](https://helpcenter.duve.com/hc/en-us/articles/28504569183389-Contactless-Online-Check-in-Flow), [Canary arrivals](https://www.canarytechnologies.com/products/hotel-arrival) | Add room-ready status and QR entry first; integrate keys only through an auditable provider boundary |
| Guest communication | In-app AI/reception conversation and realtime events | Duve supports lifecycle messaging across WhatsApp, SMS, email, and its hub with fallback rules; HiJiffy positions multilingual omnichannel automation across the guest journey. [Duve scheduled-message fallback](https://helpcenter.duve.com/hc/en-us/articles/20359199801373-Scheduled-Messages-Logic-for-SMS-Fallback-after-WhatsApp-fails), [HiJiffy overview](https://www.hijiffy.com/wp-content/uploads/2025/03/HiJiffy-Info-pack-2025-ENGLISH.pdf) | Add consent-aware channel adapters, scheduled triggers, delivery evidence, fallback, and one staff inbox |
| Context and personalization | Static hotel content, quick actions, and tenant-level service availability | Duve exposes time/reservation-conditioned guest shortcuts and recommended offers. [Duve customized shortcuts](https://helpcenter.duve.com/hc/en-us/articles/30007291548189-New-Customized-Shortcuts-for-the-Guest-App) | Rank actions, information, and offers by stay stage, inventory, room/party context, and explicit preference |
| Ordering and upsells | One-action free/paid requests with pay-at-hotel totals | Duve supports scheduled mobile orders, inventory limits, flexible charges, payment links, room-upgrade availability, and folio updates. [Duve product updates](https://helpcenter.duve.com/hc/en-us/articles/7769709812765-Duve-Product-Updates) | Introduce catalog detail, options, slots, inventory, basket, payments, and reconciliation without weakening operational simplicity |
| Checkout and recovery | Staff checkout and guest session revocation | Duve and Canary position digital checkout within the same guest journey; Canary also includes feedback/tipping in its guest-management suite. [Duve for hotels](https://duve.com/hotels/), [Canary guest management](https://us.canarytechnologies.com/products/guest-management-software) | Add bill review, payment/receipt, issue reporting, feedback, and targeted service recovery before session closure |
| Integrations | Internal source of truth only | Duve and Canary advertise PMS, payments, POS, and mobile-key integrations as core arrival/commerce enablers. [Duve pricing and integrations](https://duve.com/pricing/), [Canary mobile check-in](https://www.canarytechnologies.com/products/contactless-check-in) | Build idempotent connector contracts and exception queues before connecting a specific vendor |

## Product principles for every milestone

1. Keep the guest journey web-first and usable without an app-store install.
2. Minimize authentication and form friction without weakening reservation privacy or tenant boundaries.
3. Ask for data only when it has an approved operational, legal, personalization, or payment purpose.
4. Make automation observable and reversible; staff must see why a guest or integration is blocked.
5. Keep vendor-specific behavior behind capabilities and adapters rather than embedding one PMS, channel, key, or PSP into the domain model.
6. Design Persian RTL first while making layout, content, dates, numbers, and contracts locale-aware.
7. Measure completion, time, errors, operational effort, satisfaction, and revenue—not only feature usage.
8. Ship accessible loading, empty, error, offline, expired-link, and partial-progress states as part of the feature.

## M8 — Online check-in 2.0 and arrival readiness

### Outcome

An invited guest can complete the hotel-defined arrival formalities from one secure link, pause and resume safely, correct rejected steps, and arrive with the front desk already aware of readiness and exceptions.

### Scope

- Signed, expiring, revocable reservation invitations and QR recovery that do not expose guest identity in URLs or require a hotel slug.
- Configurable per-hotel check-in templates with ordered, required/optional, conditional, and localized steps.
- Booker and companion profiles, contact details, nationality/date fields only where required, arrival time/method, transport, accessibility needs, special requests, and hotel-defined questions.
- Per-guest camera/file document collection, malware/content validation, replacement, explicit purpose copy, retention policy, and optional OCR/MRZ provider interface with manual fallback.
- Versioned registration card/terms, locale-specific contract text, signer rules, consent evidence, e-signature artifact, withdrawal/correction handling, and immutable audit history.
- Draft autosave, cross-device resume, progress indicator, validation summaries, accessible error focus, link expiry/recovery, and safe retry/idempotency.
- A check-in state machine covering draft, submitted, needs changes, approved, arrival pending, room ready, checked in, expired, and cancelled.
- Staff arrivals workspace with completeness score, outstanding steps, risk/verification state, arrival ETA, filters, bulk reminders, review ownership, reasoned rejection, and room-ready action.
- A feature-flagged payment/deposit step contract that M11 can fulfill without redesigning the check-in state machine.
- Step-level analytics for invitation, open, start, save, error, abandon, submit, review, resubmit, approve, room ready, and physical arrival.

### Acceptance evidence

- Tenant/RBAC and lifecycle migrations, OpenAPI, retention/audit behavior, and integration tests cover every state transition and cross-tenant attempt.
- End-to-end tests cover a single guest, companions, optional steps, rejected document/signature correction, expired/revoked links, resume, cancellation, and checked-in replay.
- No identity document, signature, token, or sensitive answer appears in logs, analytics payloads, URLs, or unauthorized API responses.
- A moderated mobile usability test completes the required flow without assistance for at least five of six representative participants; median guest input time is under five minutes when manual review time is excluded.
- Pilot dashboards expose completion rate, step abandonment, technical failure, document rework, median completion time, and front-desk handling time. Product owners approve pilot targets before general availability.
- Automated accessibility checks and manual keyboard/screen-reader review find no critical WCAG 2.2 AA blockers in the invitation, wizard, review, and recovery paths.

## M9 — Guest and staff experience redesign

### Outcome

Guests and staff can understand what matters now, complete frequent tasks with less effort, and recover confidently from errors across mobile, tablet, and desktop.

### Scope

- Shared design tokens, typography, icons, spacing, components, form patterns, motion rules, focus behavior, and documented loading/empty/error/success states.
- Lifecycle-aware guest home for upcoming stay, arrival readiness, room readiness/access, current stay, checkout, and post-stay feedback.
- Context-sensitive shortcuts for check-in, Wi-Fi, room/access information, hotel guide, active requests, next booking, transport, and support.
- Service search/categories, rich details, availability, quantity, modifiers, delivery location/time, confirmation, duplicate prevention, and accessible bottom-sheet/full-page patterns.
- Separate active requests, scheduled itinerary, and history; expose owner, promised window, timeline, cancellation rules, reorder, receipt, and escalation.
- Searchable digital hotel directory with facilities, menus, policies, maps/directions, emergency information, accessibility information, and offline-safe essential content.
- Staff dashboards tailored by reception, housekeeping, F&B, operations, and administrator roles with SLA aging, saved filters, ownership, bulk actions, keyboard support, and mobile task views.
- Persian, English, and Arabic locale architecture, including RTL/LTR switching, translated content fallback, localized contracts, dates, numbers, currency, and validation messages.
- Installable PWA shell, sensible offline/reconnect behavior, skeleton loading, stale-state indicators, cache/version recovery, and deep-link preservation.

### Acceptance evidence

- The component library has visual regression, interaction, responsive, RTL/LTR, and accessibility coverage.
- Core guest tasks—check-in continuation, finding Wi-Fi, placing a configured order, tracking it, messaging staff, and checkout—pass scenario-based usability tests with defined success/time/error targets.
- Core staff tasks—triaging arrivals, accepting/assigning work, finding an overdue request, resolving an exception, and answering a guest—pass role-specific usability tests.
- WCAG 2.2 AA checks include keyboard, focus, screen reader, zoom/reflow, contrast, reduced motion, touch targets, and localized content.
- Production telemetry measures Core Web Vitals; the launch budget targets p75 LCP at or below 2.5 seconds, INP at or below 200 milliseconds, and CLS at or below 0.1 on supported guest devices.

## M10 — Lifecycle communication and personalization

### Outcome

The right guest receives the right operational message or relevant action at the right stage, through a consented channel, while staff work from one accountable conversation queue.

### Scope

- Provider-neutral in-app, email, SMS, and WhatsApp adapters with sandbox modes, health state, delivery receipts, normalized errors, retry/backoff, and cost metadata.
- Localized versioned templates, approved variables, previews, test sends, scheduling, quiet hours, frequency caps, and hotel/brand overrides.
- Triggers for confirmation, check-in invitation/reminder, incomplete step, arrival day, room ready, service status, mid-stay check, checkout, payment issue, feedback, and recovery.
- Consent purpose/channel records, source and timestamp, unsubscribe keywords/links, suppression, legal retention, and immediate enforcement across automated and manual sends.
- Channel fallback policy with idempotency so a retry or provider failure cannot produce duplicate guest messages.
- Unified staff inbox for in-app and external-channel threads with reservation context, assignment, department routing, unread/SLA state, internal notes, escalation, and audit history.
- Guest profile preferences and stay context separated from sensitive identity data; explicit controls define which roles and personalization rules may use each field.
- Dynamic shortcuts, content, and offers based on stay stage, inventory, room/party facts, language, explicit interests, and prior accepted/declined offers.

### Acceptance evidence

- Adapter contract tests simulate success, delay, rejection, timeout, duplicate callbacks, provider outage, fallback, and replay.
- Consent and suppression tests prove that opt-outs apply immediately and cannot be bypassed by templates, fallback, staff sends, or a second provider.
- Delivery, failure, fallback, response, handoff, unsubscribe, and cost metrics are visible per hotel/channel/template without exposing message bodies to analytics.
- A staging journey executes confirmation through post-stay communication with deterministic timestamps, no duplicates, and a complete audit trail.

## M11 — Commerce, payments, upsells, and digital checkout

### Outcome

Guests can purchase relevant, fulfillable services and complete checkout transparently while HotelMate maintains an auditable financial and operational record.

### Scope

- Catalog variants, modifiers, inventory/capacity, blackout rules, lead time, fulfillment slots, taxes/fees, cancellation/refund rules, and hotel-timezone pricing.
- Basket and order aggregate distinct from fulfillment tasks; changes preserve price snapshots and create auditable financial events.
- Rule-based offers for room upgrades, breakfast, transport, spa, early check-in, late checkout, and hotel-defined products, constrained by inventory and frequency caps.
- PSP capability interface for hosted payment links/fields, deposits/holds, capture, cancellation, refund, expiry, and 3-D Secure where supported.
- Idempotent signed webhooks, replay protection, reconciliation jobs, exception queue, settlement/export evidence, and no raw card storage or logging.
- Digital checkout with folio/bill presentation, disputed-item workflow, outstanding balance, payment state, receipt/invoice, checkout request, key/session lifecycle, feedback, and service recovery.
- Revenue dashboards for impressions, conversion, gross/net revenue, refunds, payment failure, fulfillment cancellation, and incremental revenue by offer/channel/stay stage.

### Acceptance evidence

- Security review documents PCI scope, threat model, provider responsibilities, key rotation, webhook verification, fraud/chargeback controls, and incident handling.
- Integration tests prove price integrity, inventory contention, duplicate submission/webhook safety, partial failure, refund/reconciliation, and tenant/currency isolation.
- The same order can be traced from offer impression through payment, fulfillment, folio/export, refund, and audit events.
- Checkout cannot close a stay with an unresolved required balance or silently lose a payment/fulfillment exception.

## M12 — Hospitality integrations, automation, and journey intelligence

### Outcome

HotelMate participates safely in the hotel's system of record, automates repeatable handoffs, and shows where the guest journey or operation needs improvement.

### Scope

- Connector SDK and capability discovery for PMS/CRS, POS, CRM, channel managers, PSPs, and mobile-key providers.
- Canonical external identifiers, per-tenant credentials, webhook verification, outbox/inbox, idempotency, ordering, rate limits, retries, dead-letter queues, replay, backfill, and reconciliation.
- Reservation/guest/room/folio import, contact/pre-check-in/order/payment/task export, conflict policy, source-of-truth ownership, and visible sync status.
- Room-ready automation and mobile-key release only when identity, payment, stay, room, and provider policies all pass; every release/revocation is audited and manually recoverable.
- Integration operations console with connector health, lag, last success, failures, affected reservations, replay controls, and least-privilege support access.
- Journey analytics for invitation-to-check-in, arrival readiness, service discovery/order/fulfillment, messaging, checkout, feedback, and return behavior.
- Operational SLA dashboards, revenue attribution, cohorts, segments, controlled experiments, guardrail metrics, privacy controls, data export/deletion, and semantic metric definitions.

### Acceptance evidence

- A contract-tested sandbox connector and one approved pilot PMS pass import, update, replay, outage, conflict, and reconciliation drills without duplicate reservations or financial events.
- A simulated key provider proves issuance, wait mode, release, expiry, revocation, phone-loss recovery, provider outage, and checkout behavior without granting unauthorized access.
- Analytics event schemas are versioned, consent/data-minimization reviewed, quality monitored, and reconciled to authoritative operational/financial records.
- Staff can find and repair a failed integration from the exception queue without direct database edits.

## Sequencing and priority

| Priority | Milestone | Dependency and rationale |
| --- | --- | --- |
| P0 | M8 Online check-in 2.0 | Highest guest and front-desk value; creates the lifecycle and analytics foundation |
| P0 | M9 Experience redesign | Can begin with M8 research/design and prevents new workflows from extending current UX inconsistencies |
| P1 | M10 Communication and personalization | Depends on stable journey events, consent model, and guest context from M8/M9 |
| P1 | M11 Commerce and checkout | Depends on service UX, identity/consent, and the operational release controls planned in M7 |
| P2 | M12 Integrations and intelligence | Starts with connector discovery earlier, but production sync follows stable domain workflows and observability |

M7 is a production-release dependency for M8–M12, particularly secrets, CI/CD, backups, monitoring, and recovery. Product discovery, design, API contracts, and test fixtures may proceed in parallel with M7.

## Discovery required before M8 implementation

1. Interview reception, housekeeping/F&B, operations management, and at least six recent guests across mobile/device/language profiles.
2. Shadow one arrival peak and quantify current desk steps, queue time, document rework, missing information, and room-ready communication.
3. Select the first property/market and document mandatory registration data, signature, identity-document, payment, tax, and retention requirements with qualified legal review.
4. Identify the hotel PMS/CRS, POS, lock, messaging, and payment vendors plus API/sandbox availability and commercial constraints.
5. Establish baseline funnels and target outcomes for check-in completion, abandonment, handling time, service response, guest satisfaction, and ancillary revenue.
6. Prototype the invitation, check-in wizard, staff arrivals queue, room-ready state, service order, and tracking history; test them before finalizing schemas.
