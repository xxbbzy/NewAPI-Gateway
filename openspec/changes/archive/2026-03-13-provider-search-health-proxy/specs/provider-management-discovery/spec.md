## ADDED Requirements

### Requirement: Provider List SHALL Support Server-Side Search
The system SHALL allow administrators to search providers from the management list using a server-side `keyword` query while preserving the unified pagination protocol.

#### Scenario: Search providers by name or base URL
- **WHEN** an administrator requests the provider list with a non-empty `keyword`
- **THEN** the system SHALL filter providers by matching the keyword against provider name or base URL before pagination is applied

#### Scenario: Search resets list navigation to the first page
- **WHEN** an administrator changes the provider search keyword in the management UI
- **THEN** the UI SHALL request the provider list from the first page instead of keeping the prior page index

#### Scenario: Search preserves paginated response envelope
- **WHEN** a filtered provider list is returned
- **THEN** the response SHALL still include `items`, `p`, `page_size`, `total`, `total_pages`, and `has_more`

### Requirement: Provider Management SHALL Expose Balance And Operations Summary
The system SHALL provide an administrator-facing provider summary that aggregates operational metrics across all provider accounts.

#### Scenario: Summary returns aggregate balance across parseable provider balances
- **WHEN** an administrator requests provider management summary data
- **THEN** the system SHALL return the sum of all parseable provider balances and the count of providers included in that sum

#### Scenario: Summary returns operational counts
- **WHEN** provider management summary data is returned
- **THEN** the system SHALL include counts for at least total providers, currently unreachable providers, and proxy-enabled providers

#### Scenario: Summary is visible in provider management
- **WHEN** an administrator opens provider management
- **THEN** the UI SHALL display the returned provider balance and operations summary before the provider table

### Requirement: Provider Management SHALL Surface Balance Freshness
The system SHALL expose whether provider balances are current enough for operations visibility.

#### Scenario: Summary includes balance freshness metadata
- **WHEN** provider management summary data is returned
- **THEN** the system SHALL include balance freshness information derived from provider balance update timestamps

#### Scenario: Provider row exposes last balance update context
- **WHEN** the provider list or detail view renders a provider with a known balance update time
- **THEN** the UI SHALL display a readable balance update time for that provider
