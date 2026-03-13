# provider-health-monitoring Specification

## Purpose
TBD - created by archiving change provider-search-health-proxy. Update Purpose after archive.

## Requirements
### Requirement: Provider SHALL Persist Operational Health State Separately From Manual Enablement
The system SHALL persist provider operational health metadata independently from administrator-controlled provider enablement state.

#### Scenario: Manual disable remains distinct from health state
- **WHEN** an administrator disables a provider manually
- **THEN** the system SHALL preserve that manual disable state without overwriting it as an automatically detected health failure

#### Scenario: Health metadata can record failure without changing manual status
- **WHEN** the system detects a provider as unreachable
- **THEN** the system SHALL record provider health failure metadata without requiring the administrator-controlled status field to be rewritten

### Requirement: Provider SHALL Be Marked Unreachable After Classified Access Failure
The system SHALL treat provider site health as a sync-derived accessibility signal and SHALL mark the provider unreachable when the latest sync run encounters a classified transport or upstream access failure.

#### Scenario: Sync run marks provider unreachable on classified access failure
- **WHEN** provider sync encounters a classified transport or upstream access failure
- **THEN** the system SHALL record the provider as unreachable with failure context and timestamp

#### Scenario: Later sync substep success does not override earlier classified failure in the same run
- **WHEN** one sync substep records a classified reachability failure and a later sync substep succeeds in the same sync run
- **THEN** the final persisted provider site health for that run SHALL remain unreachable

#### Scenario: Checkin failure does not rewrite provider site health
- **WHEN** provider checkin fails for reasons including disabled upstream checkin or human verification requirements
- **THEN** the system SHALL record the checkin result without changing provider site health

### Requirement: Unreachable Providers SHALL Stop Participating In Automated Usage
The system SHALL prevent unreachable providers from continuing to participate in automated provider usage flows until they recover or are explicitly cleared according to the configured recovery behavior.

#### Scenario: Unreachable provider is excluded from relay selection
- **WHEN** route selection evaluates candidate providers for an incoming request
- **THEN** providers marked unreachable SHALL NOT be selected for relay traffic

#### Scenario: Unreachable provider is excluded from scheduled provider jobs
- **WHEN** automatic sync or scheduled checkin considers eligible providers
- **THEN** providers marked unreachable SHALL be skipped from those automated runs

### Requirement: Provider Health SHALL Recover On Successful Subsequent Access
The system SHALL clear the unreachable state only when a later provider sync run completes without a classified site-access failure.

#### Scenario: Successful sync restores provider health
- **WHEN** a provider previously marked unreachable completes a sync run without classified access failures
- **THEN** the system SHALL mark that provider healthy again and record the successful access time

#### Scenario: Successful checkin does not independently restore provider health
- **WHEN** a provider previously marked unreachable completes a successful checkin before the next successful sync
- **THEN** the system SHALL retain the existing provider site health until sync confirms recovery

### Requirement: Provider Management SHALL Show Current Health And Last Failure Context
The system SHALL display provider site health as the latest sync-derived accessibility result and SHALL keep checkin state visible as a separate operational signal in management views.

#### Scenario: Provider list shows sync-derived site health
- **WHEN** an administrator views the provider list
- **THEN** each provider row SHALL show whether the latest sync result considers the site healthy or unreachable

#### Scenario: Provider detail separates site health and checkin state
- **WHEN** an administrator opens provider detail for a provider with recorded health and checkin metadata
- **THEN** the UI SHALL show sync-derived site health separately from checkin result fields and SHALL NOT infer site health from checkin outcome

### Requirement: Provider Health Transitions SHALL Support Configurable Notifications
The system SHALL emit notification-eligible provider health transition events when notification dispatch is enabled for provider health incidents.

#### Scenario: Provider becomes unreachable
- **WHEN** a provider health flow changes a provider into the unreachable state after a classified access failure
- **THEN** the system SHALL emit one notification-eligible unreachable event containing the provider identity and recorded failure reason

#### Scenario: Provider recovers after previous failure
- **WHEN** a provider previously marked unreachable is later marked healthy by a successful covered access
- **THEN** the system SHALL emit one notification-eligible recovery event for that provider

#### Scenario: Repeated failure during same active incident
- **WHEN** additional classified access failures occur while the provider is already in the same active unreachable incident state
- **THEN** the system SHALL NOT emit duplicate unreachable notifications for that unchanged incident state
