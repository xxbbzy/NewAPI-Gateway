## ADDED Requirements

### Requirement: Provider SHALL Persist Operational Health State Separately From Manual Enablement
The system SHALL persist provider operational health metadata independently from administrator-controlled provider enablement state.

#### Scenario: Manual disable remains distinct from health state
- **WHEN** an administrator disables a provider manually
- **THEN** the system SHALL preserve that manual disable state without overwriting it as an automatically detected health failure

#### Scenario: Health metadata can record failure without changing manual status
- **WHEN** the system detects a provider as unreachable
- **THEN** the system SHALL record provider health failure metadata without requiring the administrator-controlled status field to be rewritten

### Requirement: Provider SHALL Be Marked Unreachable After Classified Access Failure
The system SHALL classify provider-level access failures and mark the provider unreachable when those failures indicate the upstream site cannot currently be used.

#### Scenario: Sync chain marks provider unreachable on access failure
- **WHEN** provider sync encounters a classified transport or upstream access failure
- **THEN** the system SHALL record the provider as unreachable with failure context and timestamp

#### Scenario: Checkin chain marks provider unreachable on access failure
- **WHEN** provider checkin encounters a classified transport or upstream access failure
- **THEN** the system SHALL record the provider as unreachable with failure context and timestamp

#### Scenario: Relay chain marks provider unreachable on access failure
- **WHEN** upstream relay attempts for a provider encounter a classified access failure
- **THEN** the system SHALL record the provider as unreachable with failure context and timestamp

### Requirement: Unreachable Providers SHALL Stop Participating In Automated Usage
The system SHALL prevent unreachable providers from continuing to participate in automated provider usage flows until they recover or are explicitly cleared according to the configured recovery behavior.

#### Scenario: Unreachable provider is excluded from relay selection
- **WHEN** route selection evaluates candidate providers for an incoming request
- **THEN** providers marked unreachable SHALL NOT be selected for relay traffic

#### Scenario: Unreachable provider is excluded from scheduled provider jobs
- **WHEN** automatic sync or scheduled checkin considers eligible providers
- **THEN** providers marked unreachable SHALL be skipped from those automated runs

### Requirement: Provider Health SHALL Recover On Successful Subsequent Access
The system SHALL clear the unreachable state when a later provider access succeeds through a covered chain.

#### Scenario: Successful sync restores provider health
- **WHEN** a provider previously marked unreachable completes a successful sync access
- **THEN** the system SHALL mark that provider healthy again and record the successful access time

#### Scenario: Successful relay restores provider health
- **WHEN** a provider previously marked unreachable serves a successful upstream relay request
- **THEN** the system SHALL mark that provider healthy again and clear the active unreachable marker

### Requirement: Provider Management SHALL Show Current Health And Last Failure Context
The system SHALL display provider health status and recent failure context in the management experience.

#### Scenario: Provider list shows reachable vs unreachable state
- **WHEN** an administrator views the provider list
- **THEN** each provider row SHALL show whether the provider is currently healthy or unreachable

#### Scenario: Provider detail shows last failure reason
- **WHEN** an administrator opens provider detail for a provider with recorded health failure metadata
- **THEN** the UI SHALL show the latest failure reason and failure time
