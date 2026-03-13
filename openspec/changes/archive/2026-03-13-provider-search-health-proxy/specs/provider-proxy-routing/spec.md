## ADDED Requirements

### Requirement: Provider SHALL Support Independent Outbound Proxy Configuration
The system SHALL allow administrators to configure outbound proxy usage independently for each provider.

#### Scenario: Administrator enables proxy for a provider
- **WHEN** an administrator saves provider settings with proxy enabled and a proxy URL
- **THEN** the system SHALL persist that provider-specific proxy configuration

#### Scenario: Administrator disables proxy for a provider
- **WHEN** an administrator saves provider settings with proxy disabled
- **THEN** the system SHALL persist proxy disabled for that provider and SHALL NOT require a proxy URL to remain active

### Requirement: Provider Proxy Configuration SHALL Apply To Covered Upstream Access Chains
The system SHALL apply provider-specific outbound proxy settings consistently across provider sync, provider checkin, and upstream relay access.

#### Scenario: Sync uses provider proxy
- **WHEN** a provider has proxy enabled and the system performs upstream sync for that provider
- **THEN** the upstream HTTP requests SHALL use that provider's proxy configuration

#### Scenario: Checkin uses provider proxy
- **WHEN** a provider has proxy enabled and the system performs upstream checkin for that provider
- **THEN** the upstream HTTP requests SHALL use that provider's proxy configuration

#### Scenario: Relay uses provider proxy
- **WHEN** a provider has proxy enabled and the system forwards end-user traffic to that provider
- **THEN** the upstream relay request SHALL use that provider's proxy configuration

### Requirement: Sensitive Proxy Credentials SHALL Be Redacted In Management Responses
The system SHALL avoid exposing full provider proxy credentials in normal management reads, exports, and logs.

#### Scenario: Provider list omits raw proxy secret
- **WHEN** an administrator queries the provider list
- **THEN** the response SHALL indicate whether proxy is enabled without returning the full raw proxy credential URL

#### Scenario: Provider detail redacts stored proxy credentials
- **WHEN** an administrator loads provider detail for a proxy-enabled provider
- **THEN** the response SHALL redact embedded proxy credentials while still providing enough information to understand the configured endpoint

#### Scenario: Logs do not print raw proxy URL
- **WHEN** provider access through a proxy fails and the system logs the error
- **THEN** the log output SHALL NOT include the full raw proxy URL with credentials
