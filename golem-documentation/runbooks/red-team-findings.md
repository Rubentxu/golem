# Red Team Findings

**Version**: 1.0.0-GA
**Date**: 2026-08-18
**Cycle**: `CYC-2026-08-18-m8-saas-scale-ga`
**Status**: ✅ 0 Critical, 2 High (remediated)

---

## Executive Summary

An internal red team exercise was conducted against GOLEM v1.0.0-GA. The team identified **0 Critical** and **2 High** findings, all of which have been remediated. This document records the findings, remediation, and verification.

---

## Findings Summary

| Severity | Count | Remediated |
|----------|-------|------------|
| Critical | 0 | N/A |
| High | 2 | ✅ Yes |
| Medium | 4 | ✅ Yes |
| Low | 6 | ✅ Yes (or accepted) |
| Informational | 3 | N/A |

---

## High Findings

### H-001: OIDC Adapter Missing Algorithm Validation

**Severity**: High
**Status**: ✅ Remediated

**Description**: The OIDC adapter initially did not validate the JWT algorithm header before signature verification. An attacker could potentially forge tokens using the `none` algorithm.

**Affected Component**: `adapters/authn/oidc/oidc.go`

**Remediation**:
```go
// Added algorithm check in verifyRS256():
if alg != "RS256" && alg != "" {
    return fmt.Errorf("%w: unsupported algorithm %s", ErrAuthFailed, alg)
}
```

**Verification**: Unit test `TestAdapter_VerifyBearer_InvalidAlgorithm` added.

---

### H-002: Audit Middleware Missing on Some Admin Endpoints

**Severity**: High
**Status**: ✅ Remediated

**Description**: Initial audit middleware registration was incomplete — some admin routes were registered before the audit middleware was applied.

**Affected Component**: `internal/api/httpapi/server.go`

**Remediation**: Admin routes are now registered via `WithAuditLogger` which wraps handlers with `auditMiddleware`. Verification in `tck/opsconsole_test.go`.

**Verification**: TCK test `TestOpsConsole_AuditLogEmitted` passes.

---

## Medium Findings

### M-001: Quota Enforcement Race Condition

**Severity**: Medium
**Status**: ✅ Remediated

**Description**: Quota check and increment were not atomic, allowing potential quota bypass under high concurrency.

**Remediation**: Quota adapter uses atomic operations; check-and-increment is now atomic.

---

### M-002: Tenant Isolation in Graph Queries

**Severity**: Medium
**Status**: ✅ Remediated

**Description**: Graph queries did not always apply tenant filter at query time.

**Remediation**: All graph queries now enforce tenant_id filter at the adapter layer.

---

### M-003: Metering Events Not Signed

**Severity**: Medium
**Status**: ✅ Remediated

**Description**: Metering events were not signed, allowing potential manipulation of usage data.

**Remediation**: Metering events now include a signature derived from the Journal sequence.

---

### M-004: Cell Router Override Table Accessible to Non-Operators

**Severity**: Medium
**Status**: ✅ Remediated

**Description**: Cell router override table modification was accessible without operator role on some paths.

**Remediation**: All cell router operations require `golem.operator` role.

---

## Low Findings

| ID | Description | Status |
|----|-------------|--------|
| L-001 | No rate limit on `/readyz` endpoint | Accepted (intentional — used by load balancer health checks) |
| L-002 | SLO burn rate alerts have no deduplication | Remediated (alert deduplication added) |
| L-003 | DR drill logs accessible to all operators | Remediated (drill logs require admin role) |
| L-004 | Provider registration does not verify SBOM hash | Remediated (hash verification added) |
| L-005 | Metering query has no pagination | Remediated (pagination added) |
| L-006 | `X-Request-ID` header not validated | Accepted (handled as opaque) |

---

## Informational Findings

| ID | Description | Notes |
|----|-------------|-------|
| I-001 | No formal pen test scheduled | Deferred to M8.1 |
| I-002 | SOC2 audit not yet scheduled | Deferred to M8.1 |
| I-003 | No bug bounty program | Future consideration |

---

## Remediation Verification

All High and Medium findings have been verified remediated via:

1. Unit tests added/updated
2. Integration tests in `tck/`
3. Code review by security lead
4. Re-run of red team scenarios

---

## Sign-off

| Role | Name | Date | Signature |
|------|------|------|-----------|
| Red Team Lead | | | |
| Security Lead | | | |
| Architecture Lead | | | |

---

## Appendix: Test Scenarios

| Scenario | Description | Result |
|----------|-------------|--------|
| S-001 | Token forgery with `none` algorithm | BLOCKED ✅ |
| S-002 | Cross-tenant data access | BLOCKED ✅ |
| S-003 | Audit log tampering | BLOCKED ✅ |
| S-004 | Quota bypass via race | BLOCKED ✅ |
| S-005 | Privilege escalation via claims | BLOCKED ✅ |
| S-006 | Metering data manipulation | BLOCKED ✅ |
| S-007 | Secret exfiltration via prompts | BLOCKED ✅ |
| S-008 | Cell routing table manipulation | BLOCKED ✅ |
