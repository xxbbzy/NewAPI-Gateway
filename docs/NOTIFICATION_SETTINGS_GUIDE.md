# Notification Dispatch Guide

## Overview

NewAPI Gateway can send operator alerts through three outbound channels:

- Bark push notifications
- Generic webhook callbacks
- SMTP email

The notification system is intended for operational awareness rather than end-user messaging. Delivery is asynchronous and never changes the success or failure result of the originating checkin, sync, or relay workflow.

## Setup

Open the admin system settings page and expand `高级配置（默认折叠）`, then use the `运营通知` card.

### Channel Configuration

- `Bark`: provide the Bark server address and device key. Group is optional.
- `Webhook`: provide the destination URL. An optional token is sent as `Authorization: Bearer <token>`.
- `Email`: enable the notification email channel and provide one or more recipients separated by commas, semicolons, or new lines.

If you want to use email notifications, first configure the shared SMTP credentials in the `配置 SMTP` section. The notification email channel reuses that SMTP transport.

## Supported Event Types

You can independently enable or disable notifications for these event families:

- Checkin run summaries
- Provider-level checkin failures
- Provider auto-disable events triggered by upstream-disabled checkin responses
- Provider unreachable and recovery transitions
- Request-failure threshold alerts derived from persisted relay usage logs

## Concise vs Detailed

The system supports two rendering modes:

- `concise`: short summaries optimized for noisy on-call channels and mobile push banners
- `detailed`: includes additional context such as provider name, trigger source, failure category, counters, and recent failure reason summaries when available

Webhook deliveries always include structured JSON fields. Concise or detailed mode also controls the human-readable `text` field shared across Bark, webhook, and email rendering.

## Request-Failure Threshold Alerts

Request-failure alerts are aggregated instead of sending one notification per failed request.

- `请求失败阈值`: how many failed requests must be recorded before an alert is emitted
- `统计窗口（分钟）`: how far back the system looks when counting failed requests

When the threshold is reached inside the configured window, the gateway emits one aggregated alert for the provider and suppresses duplicates for the same active incident window.

## Secrets And Safety

Sensitive notification values are not returned by the options API:

- Bark device keys
- Webhook URLs
- Webhook tokens
- SMTP access credentials

If a sensitive field already has a stored value, leaving the field blank in the admin UI keeps the existing secret unchanged.
