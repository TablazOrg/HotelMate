# Product improvement milestones: M8–M12

## Purpose

This roadmap improves HotelMate from a secure functional MVP into a competitive hotel guest-experience platform. It is based on a source-code/API review, hands-on review of the local guest and staff interfaces, the supplied `hotelmate-netlify-preview` HTML prototype reviewed on 2026-08-23, and current official product material from Duve, Canary, and HiJiffy.

Competitor material is vendor-published and should be treated as evidence of category expectations, not independent proof of business impact. HotelMate should validate every proposed feature with target hotels and guests before committing to a provider or workflow.

## How the supplied prototype changes this roadmap

The prototype is the interaction and information-architecture reference for M8–M12. It defines four entry modes and an end-to-end target flow:

1. Public hotel discovery and paid-service reservation without an account.
2. Resident/pre-arrival login and a lifecycle-aware guest home.
3. Direct online check-in with contact, identity evidence, signature, and consent.
4. Authenticated staff operations covering requests, conversations, reservations, stays, content, promotions, analytics, users, and hotel settings.

The prototype does **not** prove production capability. Its executable source identifies the build as a mock API backed by browser `localStorage`; analytics are static fixtures, uploads are data/object URLs, browser tabs simulate realtime delivery, and several feature switches have no complete workflow behind them. The attachment's `INTEGRATION.md` describes a different backend-connected variant and therefore is not used as evidence that this preview is integrated. The label “STEP 1 OF 3” has only one implemented check-in screen, and the displayed signature pad is not an interactive signature capture.

Accordingly, roadmap status uses three evidence levels:

- **Implemented:** behavior exists in the HotelMate repository and passes its required tests.
- **Prototype-defined:** the supplied preview establishes desired UX, copy, state, or workflow, but still requires secure domain/API implementation.
- **Planned:** neither repository behavior nor the supplied preview closes the capability.

### Prototype-to-milestone traceability

| Prototype surface or rule | Owning milestone | Roadmap treatment |
| --- | --- | --- |
| Public, resident, online-check-in, and staff entry modes | M8 + M9 | Preserve the mental model; replace manual identifiers with secure invitation/deep-link and recovery paths |
| Name, booking reference, phone, identity type/number, multiple document tiles, consent, and signature | M8 | Build a real resumable multi-step journey with validated uploads, versioned terms/signature, review, and correction |
| Confirmed reservation creates a pre-arrival stay; free requests are blocked until arrival | M8 + M11 | Make lifecycle and entitlement rules server-authoritative, explicit, tested, and configurable |
| Public hotel guide, facilities, restaurants, activities, promotions, resident home, and request tracking | M9 | Use as the guest IA reference; add search, structured detail, accessibility, recovery states, and lifecycle adaptation |
| Staff dashboard, requests, conversations, notifications, reservations, stays, catalog, knowledge, promotions, users, and settings | M9 | Use as the workspace inventory; split by role and enforce every permission on the server |
| Multilingual AI detail collection, language-neutral numerals, reception handoff, and translation hints | M10 | Productize through approved knowledge, confidence/safety controls, persisted conversations, and measured handoff |
| Browser notifications, request updates, one-hour reminders, and pre-arrival automation switches | M10 | Replace tab/local timers with consented provider delivery, schedules, idempotency, fallback, and evidence |
| Public lead capture and guest request → reception quote → guest confirm/cancel → fulfillment | M11 | Preserve as the minimum paid-order state machine; add pricing integrity, inventory, payment, folio, and reconciliation |
| Promotion cards, sticky/popup offers, eligibility, and hotel feature switches | M11 + M12 | Add frequency caps, targeting, dependencies, audit history, rollout safety, and experiment guardrails |
| Operations, AI, VAS, satisfaction, and review cards | M12 | Replace fixture values with governed events, reconciled metrics, funnels, and actionable drill-downs |

## Current HotelMate baseline

HotelMate already has valuable foundations:

- A branded, no-download Persian RTL web experience for public visitors, pre-arrival guests, active guests, staff departments, and administrators.
- Tenant-bound reservation login, pre-arrival stays, secure identity-document upload/review, check-in/check-out, and room-state transitions.
- Six quick requests, paid and pre-arrival services, live request tracking, role-filtered fulfillment queues, and persisted request history.
- Public facilities, promotions, restaurant/menu content, an approved-knowledge assistant, reception handoff, reporting, and audit administration.
- PostgreSQL integration tests, explicit migrations, private document retention, OpenAPI, Docker release paths, smoke checks, and end-to-end acceptance.

The local UX and code review also exposed product gaps:

- Guest entry requires a hotel slug plus reservation/room identifiers; there are no secure invitation links, magic links, phone verification, or on-property QR recovery.
- The repository's online check-in currently uploads one PDF/JPEG/PNG and supports staff approval/rejection. The prototype adds contact, identity, consent, signature, and multiple upload affordances, but implements them as one local-only screen without real reservation proof, companion/arrival details, interactive signature capture, secure upload, correction, or resume.
- The pre-arrival timeline explicitly shows arrival time as a future step, and staff see document status rather than overall arrival readiness and exceptions.
- The prototype clarifies pre-arrival entitlements—eligible paid orders before arrival and complimentary operational requests after check-in—but those rules and hotel feature dependencies require authoritative policy enforcement and auditability.
- Service requests are fast, and the prototype adds conversational detail collection plus a reception quote/guest-confirmation loop. There is still no complete product/variant model, basket, availability inventory, price integrity, payment, folio, or receipt.
- Request tracking mixes active and historic items in a long feed; repeated requests are hard to distinguish and there is no itinerary for scheduled services.
- Chat is in-app only. Prototype browser notifications and local one-hour reminders do not provide durable scheduling, background delivery, fallback state, WhatsApp/SMS/email adapters, opt-out workflow, or a unified cross-channel inbox.
- Content and upsells are hotel-scoped but not personalized by stay stage, guest preference, inventory, room type, party, or prior behavior.
- Payments remain “pay at hotel”; there is no PSP boundary, deposit/hold, payment link, refund, reconciliation, folio, or digital checkout.
- There is no PMS/CRS, POS, CRM, channel-manager, or digital-key connector contract.
- Operational reports measure outcomes but not guest-journey funnels, abandonment, time-on-step, message delivery, SLA aging, or offer conversion; prototype analytics values are static and cannot be used as evidence.
- The UI is responsive and visually coherent, and the prototype demonstrates broader multilingual assistant copy, but accessibility, locale-complete operation, real-user usability, performance budgets, offline recovery, and design-system consistency have not been independently verified.

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

An invited guest can complete the hotel-defined arrival formalities from one secure link, pause and resume safely, correct rejected steps, and arrive with the front desk already aware of readiness and exceptions. The production flow preserves the prototype's low-friction direct-entry concept without trusting a booking reference or identity fields by themselves.

### Scope

- Signed, expiring, revocable reservation invitations and QR recovery that do not expose guest identity in URLs or require a hotel slug.
- A clear three-step default journey derived from the prototype—guest/reservation details, documents/registration, and review/submit—with configurable per-hotel ordered, required/optional, conditional, and localized steps.
- Booker and companion profiles, contact details, nationality/date fields only where required, arrival time/method, transport, accessibility needs, special requests, and hotel-defined questions.
- Per-guest camera/file document collection for front/back or multiple evidence files, malware/content validation, replacement, explicit purpose copy, retention policy, and optional OCR/MRZ provider interface with manual fallback.
- Versioned registration card/terms, locale-specific contract text, signer rules, consent evidence, e-signature artifact, withdrawal/correction handling, and immutable audit history.
- Draft autosave, cross-device resume, progress indicator, validation summaries, accessible error focus, link expiry/recovery, and safe retry/idempotency.
- A check-in state machine covering draft, submitted, needs changes, approved, arrival pending, room ready, checked in, expired, and cancelled.
- Server-authoritative reservation confirmation → pre-arrival stay conversion plus an entitlement policy that allows eligible paid ordering before arrival while withholding in-room complimentary operations until physical check-in.
- Staff arrivals workspace with completeness score, outstanding steps, risk/verification state, arrival ETA, filters, bulk reminders, review ownership, reasoned rejection, and room-ready action.
- Audited hotel controls for online check-in and digital registration, plus a feature-flagged payment/deposit step contract that M11 can fulfill without redesigning the check-in state machine.
- Step-level analytics for invitation, open, start, save, error, abandon, submit, review, resubmit, approve, room ready, and physical arrival.

### Acceptance evidence

- Tenant/RBAC and lifecycle migrations, OpenAPI, retention/audit behavior, and integration tests cover every state transition and cross-tenant attempt.
- End-to-end tests cover a single guest, companions, optional steps, rejected document/signature correction, expired/revoked links, resume, cancellation, and checked-in replay.
- End-to-end entitlement tests prove that a confirmed pre-arrival guest can access allowed offers but cannot create complimentary in-room requests until check-in; disabled hotel controls fail closed across both UI and API.
- No identity document, signature, token, or sensitive answer appears in logs, analytics payloads, URLs, or unauthorized API responses.
- A moderated mobile usability test completes the required flow without assistance for at least five of six representative participants; median guest input time is under five minutes when manual review time is excluded.
- Pilot dashboards expose completion rate, step abandonment, technical failure, document rework, median completion time, and front-desk handling time. Product owners approve pilot targets before general availability.
- Automated accessibility checks and manual keyboard/screen-reader review find no critical WCAG 2.2 AA blockers in the invitation, wizard, review, and recovery paths.

## M9 — Guest and staff experience redesign

### Outcome

Guests and staff can understand what matters now, complete frequent tasks with less effort, and recover confidently from errors across mobile, tablet, and desktop.

### Scope

- Shared design tokens, `IRANYekanXFaNum, sans-serif` typography, icons, spacing, components, form patterns, motion rules, focus behavior, and documented loading/empty/error/success states.
- Preserve the prototype's four understandable entry choices—public discovery, resident login, online check-in, and staff login—while adapting the visible choices to hotel configuration and guest lifecycle.
- Productize the public guide hierarchy: hotel hero and assistant entry, eligible promotions, facilities, restaurants/menus, activities, hotel contact, and paid-service lead capture without exposing resident-only operations.
- Lifecycle-aware guest home for upcoming stay, arrival readiness, room readiness/access, current stay, checkout, and post-stay feedback.
- Context-sensitive shortcuts for check-in, Wi-Fi, room/access information, hotel guide, active requests, next booking, transport, support, promotions, and public content; the pre-arrival banner must explain what is and is not available before arrival.
- Service search/categories, rich details, availability, quantity, modifiers, delivery location/time, confirmation, duplicate prevention, and accessible bottom-sheet/full-page patterns.
- Separate active requests, scheduled itinerary, and history; expose owner, promised window, timeline, cancellation rules, reorder, receipt, and escalation.
- Searchable digital hotel directory with facilities, menus, policies, maps/directions, emergency information, accessibility information, and offline-safe essential content.
- Staff dashboards tailored by reception, housekeeping, F&B, operations, and administrator roles with SLA aging, saved filters, ownership, bulk actions, keyboard support, and mobile task views. The prototype's full section inventory is retained, but unauthorized sections are removed from navigation and rejected by the server rather than merely filtered in the browser.
- Tenant branding for hotel name, welcome copy, logo, hero image, primary/accent colors, and content imagery with safe upload processing, contrast validation, preview, rollback, and audit history.
- Persian, English, and Arabic locale architecture, including RTL/LTR switching, translated content fallback, localized contracts, dates, numbers, currency, and validation messages.
- Installable PWA shell, sensible offline/reconnect behavior, skeleton loading, stale-state indicators, cache/version recovery, and deep-link preservation.

### Acceptance evidence

- The component library has visual regression, interaction, responsive, RTL/LTR, and accessibility coverage.
- Core guest tasks—check-in continuation, finding Wi-Fi, placing a configured order, tracking it, messaging staff, and checkout—pass scenario-based usability tests with defined success/time/error targets.
- Core staff tasks—triaging arrivals, accepting/assigning work, finding an overdue request, resolving an exception, and answering a guest—pass role-specific usability tests.
- Prototype-regression journeys cover each entry mode and every staff workspace using production-shaped fixtures; no acceptance test may rely on `localStorage`, static analytics, shared browser-tab state, or mock credentials.
- WCAG 2.2 AA checks include keyboard, focus, screen reader, zoom/reflow, contrast, reduced motion, touch targets, and localized content.
- Production telemetry measures Core Web Vitals; the launch budget targets p75 LCP at or below 2.5 seconds, INP at or below 200 milliseconds, and CLS at or below 0.1 on supported guest devices.

## M10 — Lifecycle communication and personalization

### Outcome

The right guest receives the right operational message or relevant action at the right stage, through a consented channel, while staff work from one accountable conversation queue.

### Scope

- Provider-neutral in-app, web-push, email, SMS, and WhatsApp adapters with sandbox modes, health state, delivery receipts, normalized errors, retry/backoff, and cost metadata.
- A multilingual assistant contract based on the prototype's useful behavior: language selection persists across numeric-only replies; Persian, Arabic-Indic, and English digits normalize without changing language; structured service fields are extracted from natural input; and only missing required fields are requested.
- Localized versioned templates, approved variables, previews, test sends, scheduling, quiet hours, frequency caps, and hotel/brand overrides.
- Triggers for confirmation, check-in invitation/reminder, incomplete step, arrival day, room ready, service status, a configurable pre-service reminder defaulting to the prototype's one-hour lead time, mid-stay check, checkout, payment issue, feedback, and recovery.
- Consent purpose/channel records, source and timestamp, unsubscribe keywords/links, suppression, legal retention, and immediate enforcement across automated and manual sends.
- Channel fallback policy with idempotency so a retry or provider failure cannot produce duplicate guest messages.
- Unified staff inbox for in-app and external-channel threads with reservation context, assignment, department routing, unread/SLA state, internal notes, escalation, audit history, and clearly labeled translation assistance that never replaces the source message.
- Guest profile preferences and stay context separated from sensitive identity data; explicit controls define which roles and personalization rules may use each field.
- Dynamic shortcuts, content, and offers based on stay stage, inventory, room/party facts, language, explicit interests, and prior accepted/declined offers.

### Acceptance evidence

- Adapter contract tests simulate success, delay, rejection, timeout, duplicate callbacks, provider outage, fallback, and replay.
- Consent and suppression tests prove that opt-outs apply immediately and cannot be bypassed by templates, fallback, staff sends, or a second provider.
- Conversation tests cover Persian and supported non-Persian requests, mixed numeral systems, multi-detail utterances, missing-field follow-up, repeat-information avoidance, low-confidence handoff, emergency escalation, and staff return-to-assistant closure.
- Delivery, failure, fallback, response, handoff, unsubscribe, and cost metrics are visible per hotel/channel/template without exposing message bodies to analytics.
- A staging journey executes confirmation through post-stay communication with deterministic timestamps, no duplicates, and a complete audit trail.

## M11 — Commerce, payments, upsells, and digital checkout

### Outcome

Guests can purchase relevant, fulfillable services and complete checkout transparently while HotelMate maintains an auditable financial and operational record.

### Scope

- Catalog variants, modifiers, inventory/capacity, blackout rules, lead time, fulfillment slots, taxes/fees, cancellation/refund rules, and hotel-timezone pricing.
- A minimum paid-order state machine derived from the prototype: intent captured → details complete → awaiting hotel quote → quote sent with cost/date/time → guest adjusted/confirmed or cancelled → fulfillment in progress → completed/refunded. Every transition is idempotent and audited.
- Public paid-service lead capture that requests only name and contact details at conversion; complimentary services remain unavailable to anonymous visitors and can never be relabeled as paid leads.
- Basket and order aggregate distinct from conversations and fulfillment tasks; changes preserve price snapshots and create auditable financial events.
- Rule-based offers for room upgrades, breakfast, transport, spa, early check-in, late checkout, and hotel-defined products, constrained by lifecycle eligibility, inventory, prior accept/decline behavior, and frequency caps across cards, sticky placements, popups, and messages.
- PSP capability interface for hosted payment links/fields, deposits/holds, capture, cancellation, refund, expiry, and 3-D Secure where supported.
- Idempotent signed webhooks, replay protection, reconciliation jobs, exception queue, settlement/export evidence, and no raw card storage or logging.
- Digital checkout with folio/bill presentation, disputed-item workflow, outstanding balance, payment state, receipt/invoice, checkout request, key/session lifecycle, feedback, and service recovery.
- Revenue dashboards for impressions, conversion, gross/net revenue, refunds, payment failure, fulfillment cancellation, and incremental revenue by offer/channel/stay stage.

### Acceptance evidence

- Security review documents PCI scope, threat model, provider responsibilities, key rotation, webhook verification, fraud/chargeback controls, and incident handling.
- Integration tests prove price integrity, inventory contention, duplicate submission/webhook safety, partial failure, refund/reconciliation, and tenant/currency isolation.
- End-to-end tests cover resident, pre-arrival, and public-lead variants of the quote loop, including guest-edited date/time, stale or replaced quotes, expiry, cancellation, duplicate confirmation, staff reassignment, and complimentary-service boundary enforcement.
- The same order can be traced from offer impression through payment, fulfillment, folio/export, refund, and audit events.
- Checkout cannot close a stay with an unresolved required balance or silently lose a payment/fulfillment exception.

## M12 — Hospitality integrations, automation, and journey intelligence

### Outcome

HotelMate participates safely in the hotel's system of record, automates repeatable handoffs, and shows where the guest journey or operation needs improvement.

### Scope

- Connector SDK and capability discovery for PMS/CRS, POS, CRM, channel managers, PSPs, and mobile-key providers.
- Canonical external identifiers, per-tenant credentials, webhook verification, outbox/inbox, idempotency, ordering, rate limits, retries, dead-letter queues, replay, backfill, and reconciliation.
- Reservation/guest/room/folio import, contact/pre-check-in/order/payment/task export, conflict policy, source-of-truth ownership, and visible sync status.
- Dependency-aware, audited capability controls for online check-in, digital registration, pre-arrival paid ordering, promotions, reminders, contactless checkout, and automated pre-arrival messaging. A control cannot be enabled when required provider, consent, security, or operational readiness checks fail.
- Room-ready automation and mobile-key release only when identity, payment, stay, room, and provider policies all pass; every release/revocation is audited and manually recoverable.
- Integration operations console with connector health, lag, last success, failures, affected reservations, replay controls, and least-privilege support access.
- Journey analytics for invitation-to-check-in, arrival readiness, service discovery/order/fulfillment, quote acceptance, messaging, checkout, feedback, review conversion, and return behavior.
- Operational SLA dashboards, AI resolution/handoff, guest satisfaction, VAS value, upsell acceptance, revenue attribution, cohorts, segments, controlled experiments, guardrail metrics, privacy controls, data export/deletion, and semantic metric definitions. Every card shown in the prototype must drill into reconciled source events rather than fixture totals.

### Acceptance evidence

- A contract-tested sandbox connector and one approved pilot PMS pass import, update, replay, outage, conflict, and reconciliation drills without duplicate reservations or financial events.
- A simulated key provider proves issuance, wait mode, release, expiry, revocation, phone-loss recovery, provider outage, and checkout behavior without granting unauthorized access.
- Analytics event schemas are versioned, consent/data-minimization reviewed, quality monitored, and reconciled to authoritative operational/financial records.
- Capability-control tests prove dependency validation, staged rollout, immediate fail-closed behavior, audit attribution, rollback, and consistent enforcement in guest UI, staff UI, API, scheduled jobs, and provider callbacks.
- Staff can find and repair a failed integration from the exception queue without direct database edits.

## Sequencing and priority

| Priority | Milestone | Dependency and rationale |
| --- | --- | --- |
| P0 | M8 Online check-in 2.0 | Productizes the prototype's clearest incomplete journey and creates lifecycle, entitlement, and analytics foundations |
| P0 | M9 Experience redesign | Turns the prototype's guest/staff IA into an accessible system and can proceed alongside M8 discovery |
| P1 | M10 Communication and personalization | Productizes prototype assistant, handoff, notification, and reminder behavior after stable journey events and consent exist |
| P1 | M11 Commerce and checkout | Productizes the prototype quote loop, then adds payments; depends on service UX, lifecycle policy, and M7 release controls |
| P2 | M12 Integrations and intelligence | Starts with connector discovery earlier, but production sync follows stable domain workflows and observability |

M7 is a production-release dependency for M8–M12, particularly secrets, CI/CD, backups, monitoring, and recovery. Product discovery, design, API contracts, and test fixtures may proceed in parallel with M7.

### Prototype adoption order

The prototype should be absorbed as tested vertical slices, not copied into production as one frontend release:

| Slice | Included prototype journey | Exit condition |
| --- | --- | --- |
| 8A | Secure invitation/direct check-in entry and confirmed-reservation conversion | Reservation proof, tenant isolation, expiry/revocation, and pre-arrival entitlement tests pass |
| 8B | Details, documents, signature/consent, submission, staff review, and correction | True multi-step resume plus protected evidence, audit, retention, and arrival-readiness acceptance pass |
| 9A | Public guide, resident home, quick actions, promotions, request tracking, and branded navigation | Responsive/accessibility/usability gates pass for public, pre-arrival, and active-stay modes |
| 9B | Staff dashboard and role workspaces | Each role sees only authorized navigation/data/actions and completes its core tasks on desktop/tablet/mobile |
| 10A | Multilingual detail collection, AI handoff, staff reply, request updates, and reminders | Safety, mixed-numeral, persistence, consent, scheduling, delivery, and fallback tests pass |
| 11A | Public/resident paid intent, hotel quote, guest confirm/cancel, and fulfillment | One auditable order state machine replaces browser-tab/localStorage coordination |
| 11B | Inventory, payment, folio, receipt, and contactless checkout | PCI, reconciliation, exception, refund, and unresolved-balance gates pass |
| 12A | Provider synchronization, governed feature rollout, and real journey analytics | Pilot connector, data-quality, metric-reconciliation, dependency, and rollback evidence pass |

## Discovery required before M8 implementation

1. Interview reception, housekeeping/F&B, operations management, and at least six recent guests across mobile/device/language profiles.
2. Shadow one arrival peak and quantify current desk steps, queue time, document rework, missing information, and room-ready communication.
3. Select the first property/market and document mandatory registration data, signature, identity-document, payment, tax, and retention requirements with qualified legal review.
4. Identify the hotel PMS/CRS, POS, lock, messaging, and payment vendors plus API/sandbox availability and commercial constraints.
5. Establish baseline funnels and target outcomes for check-in completion, abandonment, handling time, service response, guest satisfaction, and ancillary revenue.
6. Use the supplied HTML as the first hypothesis, then prototype the missing invitation, true multi-step check-in, staff arrivals queue, room-ready state, structured service order, quote expiry/revision, and active/history tracking. Test all entry modes before finalizing schemas.
