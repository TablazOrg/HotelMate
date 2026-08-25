# Milestone 8: online check-in 2.0 and arrival readiness

## Delivery status

Milestone 8 is engineering-complete and its code and migration are present in the current production release. M7 production operations are closed; public enablement of the M8 guest journey remains gated by the human pilot and approval evidence listed below.

## Guest journey

1. Authorized reception or operations staff creates a purpose-bound invitation for a confirmed reservation. The signed capability and recovery code expire, can be revoked, are stored only as SHA-256 hashes, and contain no guest identity. QR recovery uses a URL fragment that the React entry page removes before the first server request; exchange occurs only in a JSON request body.
2. Exchange creates or resumes the reservation's pre-arrival stay and arrival journey. Hotel controls fail closed until online check-in and digital registration are both explicitly enabled.
3. The guest completes three server-backed steps: arrival/contact details and companions, multiple guest/companion evidence files plus versioned consent/signature, and final review. Partial details autosave, the journey resumes across sessions, and submission requires an idempotency key.
4. Reception can request a reasoned correction, approve, mark the guest arrival-pending, and announce room-ready only while the assigned room is available. Physical check-in atomically closes the journey as `checked_in`.

The complete state set is `draft`, `submitted`, `needs_changes`, `approved`, `arrival_pending`, `room_ready`, `checked_in`, `expired`, and `cancelled`. Active unfinished journeys automatically become `expired` after their retention boundary.

## Evidence and privacy

- Each guest or required companion can have multiple JPEG, PNG, or PDF evidence files.
- Storage is private and tenant-scoped. API views never expose storage keys, hashes, signature bytes, invitation hashes, or raw analytical answers.
- Content is MIME-sniffed and size-limited. EICAR signatures and PDFs containing active JavaScript, launch actions, open actions, or embedded files are rejected before metadata persistence.
- `identity.Provider` is the optional OCR/MRZ/identity-verification boundary. M8 ships a deterministic manual-review provider; a production verifier can be added without changing the journey or HTTP contracts.
- Signatures are version-bound to the hotel's current terms and locale. A terms change forces explicit re-consent and re-signing. The UI supports drawing and a keyboard-operable typed-signature alternative.
- Document/signature retention is integrated into the existing purge command and private-storage deletion path.

## Staff workspace and metrics

The arrivals workspace provides completeness, outstanding evidence/signature state, risk/verification state, arrival ETA, owner/unassigned filtering, assignment, reasoned correction, approval, arrival-pending, room-ready, individual/bulk reminder intent, and authorized evidence download. Reminder channel delivery stays behind the M10 provider boundary.

Tenant-scoped analytics expose invitations, opens, starts, submits/resubmits, corrections, approvals, room-ready, physical arrival, abandonment, technical failures, completion rate, document rework rate, median guest completion time, median review time, and step-event counts. Event payloads contain no answers, document data, signatures, or capability values.

## Rollout controls

New and existing hotels default to:

- `onlineCheckInEnabled=false`
- `digitalRegistrationEnabled=false`
- `paymentStepEnabled=false`

The optional payment step is represented by a provider-neutral `deferred_to_m11` contract and cannot collect money. Only primary or secondary administrators can change rollout controls and versioned terms; the action is audited.

## API and migration

- Migration: `2026082308_arrival_readiness`
- Contract: [OpenAPI](openapi.yaml)
- Primary implementation: `backend/internal/store/arrival.go`, `backend/internal/httpapi/handlers_arrival.go`, and `frontend/src/ArrivalExperience.tsx`
- Stateful release acceptance: `backend/internal/acceptance/suite.go`

## Development deployment

- Web: `http://127.0.0.1:43000`
- API: `http://127.0.0.1:48080`
- Release manifest: `.hotelmate/releases/0.8.0-local.json`
- Deployment evidence: `.hotelmate/evidence/development/20260824T045502.025205000Z-succeeded.json`

Ports differ from the defaults because unrelated local verification stacks already occupied `3000`, `8080`, and the lower alternate ports. The release-specific protected environment file records the selected ports.

## General-availability gates

The implementation and automated acceptance are complete. These non-engineering claims must still be produced before public general availability:

- a moderated mobile study in which at least five of six representative participants finish without assistance;
- product-owner approval of pilot completion, abandonment, failure, rework, completion-time, and front-desk handling targets;
- manual keyboard and screen-reader review across the invitation, wizard, review, correction, and recovery paths;
- feature-controlled enablement through the approved M7 signed-image production path, with the two accepted M7 operational exceptions kept visible to the rollout owner.

No document marks those gates passed without the corresponding participants and owner approval.
