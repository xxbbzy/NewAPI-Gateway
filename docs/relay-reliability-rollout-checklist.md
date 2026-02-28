# Relay Reliability Rollout Checklist

## Pre-rollout

- Ensure database migration is applied by starting the service once on each environment (auto-migrate + usage log index bootstrap).
- Verify startup preflight passes with reliability flags enabled.
- Confirm options are configured:
  - `RelayResponseValidityGuardEnabled=true`
  - `RoutingInvalidResponseSuppressionEnabled` (enable gradually)
  - `RoutingInvalidResponseSuppressionThreshold`
  - `RoutingInvalidResponseSuppressionWindowMinutes`
  - `RoutingInvalidResponseSuppressionCooldownMinutes`

## Rollout stages

1. Enable `RelayResponseValidityGuardEnabled=true` in staging and verify fallback still returns successful responses.
2. Observe invalid-response metrics for at least one full traffic cycle.
3. Enable suppression options in staging with conservative thresholds.
4. Roll out to production by traffic slice or environment wave.

## Monitoring queries

### `false_success_rate`

```sql
SELECT
  SUM(CASE WHEN status = 1 AND failure_category = 'invalid_response' THEN 1 ELSE 0 END) AS false_success,
  SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END) AS total_success,
  CASE WHEN SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END) = 0 THEN 0
       ELSE 1.0 * SUM(CASE WHEN status = 1 AND failure_category = 'invalid_response' THEN 1 ELSE 0 END)
            / SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END)
  END AS false_success_rate
FROM usage_logs
WHERE created_at >= strftime('%s','now') - 3600;
```

### `empty_2xx_rate`

```sql
SELECT
  SUM(CASE WHEN failure_category = 'invalid_response' AND invalid_reason IN ('no_actionable_output','stream_no_meaningful_delta') THEN 1 ELSE 0 END) AS empty_2xx,
  COUNT(*) AS total_attempts,
  CASE WHEN COUNT(*) = 0 THEN 0
       ELSE 1.0 * SUM(CASE WHEN failure_category = 'invalid_response' AND invalid_reason IN ('no_actionable_output','stream_no_meaningful_delta') THEN 1 ELSE 0 END) / COUNT(*)
  END AS empty_2xx_rate
FROM usage_logs
WHERE created_at >= strftime('%s','now') - 3600;
```

### `retry_depth`

```sql
SELECT
  relay_request_id,
  MAX(attempt_index) AS retry_depth
FROM usage_logs
WHERE created_at >= strftime('%s','now') - 3600
GROUP BY relay_request_id
ORDER BY retry_depth DESC
LIMIT 50;
```

### per-route invalid-response rate

```sql
SELECT
  provider_token_id,
  model_name,
  SUM(CASE WHEN failure_category = 'invalid_response' THEN 1 ELSE 0 END) AS invalid_count,
  COUNT(*) AS total_count,
  CASE WHEN COUNT(*) = 0 THEN 0
       ELSE 1.0 * SUM(CASE WHEN failure_category = 'invalid_response' THEN 1 ELSE 0 END) / COUNT(*)
  END AS invalid_rate
FROM usage_logs
WHERE created_at >= strftime('%s','now') - 3600
GROUP BY provider_token_id, model_name
ORDER BY invalid_rate DESC, total_count DESC;
```

## Rollback

- Disable `RelayResponseValidityGuardEnabled` to restore legacy 2xx transport-only success behavior.
- Disable `RoutingInvalidResponseSuppressionEnabled` to remove temporary route suppression.
- Keep schema/index additions in place (additive and backward-compatible).
