## 1. Provider Data Model And Query Foundations

- [x] 1.1 Extend the provider data model and persistence layer with provider health metadata and provider-specific proxy configuration fields
- [x] 1.2 Add provider read/write helpers for health state transitions, proxy redaction, and balance freshness/summary calculations
- [x] 1.3 Extend provider list querying to support server-side `keyword` filtering while preserving unified pagination behavior

## 2. Provider Management APIs

- [x] 2.1 Add an administrator provider summary API that returns aggregate balance, balance freshness, unreachable-provider count, and proxy-enabled-provider count
- [x] 2.2 Extend provider list and detail APIs to return health status, balance freshness metadata, and redacted proxy configuration fields
- [x] 2.3 Update provider create, update, import, and export flows to support provider proxy settings without leaking raw proxy credentials in normal responses

## 3. Provider Health Monitoring

- [x] 3.1 Define provider-level access failure classification and wire health-state recording into sync, checkin, and relay failure paths
- [x] 3.2 Enforce provider-level health gating so unreachable providers are skipped by automated sync/checkin flows and excluded from relay selection
- [x] 3.3 Add provider health recovery on successful subsequent sync or relay access and persist last success/failure context for management visibility

## 4. Provider-Specific Proxy Routing

- [x] 4.1 Refactor upstream HTTP client construction so provider sync and checkin requests can use provider-specific proxy settings
- [x] 4.2 Refactor relay upstream transport construction so end-user traffic uses the same provider-specific proxy settings
- [x] 4.3 Add proxy credential redaction in provider-facing responses, logs, and related error surfaces

## 5. Management UI Updates

- [x] 5.1 Add provider search controls and first-page reset behavior to the provider management list
- [x] 5.2 Add provider summary cards for aggregate balance, freshness, unreachable providers, and proxy-enabled providers
- [x] 5.3 Add provider row/detail health badges, last balance update display, and failure-context visibility
- [x] 5.4 Add provider form controls for enabling/disabling per-provider proxy and editing proxy endpoint settings safely

## 6. Verification

- [x] 6.1 Add backend tests for provider search, provider summary aggregation, health-state transitions, health gating, and proxy redaction behavior
- [x] 6.2 Add frontend tests for provider search reset behavior, summary rendering, health visibility, and proxy form interactions
- [x] 6.3 Run the relevant backend/frontend test suites and update provider-facing documentation if any management behavior or configuration expectations change
