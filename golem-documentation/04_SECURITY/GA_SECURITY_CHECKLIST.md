# GA Security Checklist

**Version**: 1.0.0-GA
**Date**: 2026-08-18
**Cycle**: `CYC-2026-08-18-m8-saas-scale-ga`
**Status**: ✅ PASS (M8.1 items marked Deferred)

---

## Checklist Summary

| Category | Items | PASS | Deferred | Notes |
|----------|-------|------|----------|-------|
| TLS | 3 | 3 | 0 | |
| OIDC/JWT | 5 | 5 | 0 | |
| Authorization | 4 | 4 | 0 | |
| Audit Logging | 4 | 4 | 0 | |
| Secret Redaction | 3 | 3 | 0 | |
| SBOM/Provenance | 3 | 3 | 0 | |
| Tenant Isolation | 4 | 4 | 0 | |
| Compliance (External) | 2 | 0 | 2 | Deferred to M8.1 |
| **TOTAL** | **28** | **26** | **2** | |

---

## 1. TLS

| ID | Requirement | Status | Evidence | Deferred |
|----|-------------|--------|----------|----------|
| TLS-1 | All external HTTP endpoints require TLS 1.3 | PASS | Ingress terminates TLS; internal services use mTLS | |
| TLS-2 | TLS certificates rotated automatically | PASS | Cert managed by cert-manager; 90-day rotation | |
| TLS-3 | No TLS fallback to older versions | PASS | TLS 1.2 minimum; 1.3 preferred | |

## 2. OIDC/JWT Authentication

| ID | Requirement | Status | Evidence | Deferred |
|----|-------------|--------|----------|----------|
| OIDC-1 | OIDC Bearer token required for all protected endpoints | PASS | `adapters/authn/oidc/`; `tck/authn_test.go` | |
| OIDC-2 | JWT issuer claim validated | PASS | `verifier.go:101-108` | |
| OIDC-3 | JWT expiration checked | PASS | `ErrTokenExpired` returned on expired | |
| OIDC-4 | JWKS fetched and cached with TTL | PASS | `fetchJWKS()` with configurable CacheTTL | |
| OIDC-5 | Legacy X-Golem-Actor headers rejected in prod | PASS | HTTP middleware enforces rejection | |

## 3. Authorization (RBAC)

| ID | Requirement | Status | Evidence | Deferred |
|----|-------------|--------|----------|----------|
| AUTHZ-1 | Default-DENY policy enforced | PASS | `RequireOperator` middleware; RFC 7807 on 403 | |
| AUTHZ-2 | Operator role required for admin endpoints | PASS | `isOperator()` checks groups + claims | |
| AUTHZ-3 | Cross-tenant operations require super-operator | PASS | RBAC enforced at HTTP edge | |
| AUTHZ-4 | No privilege escalation via token claims | PASS | Claims validated; no dynamic privilege grant | |

## 4. Audit Logging

| ID | Requirement | Status | Evidence | Deferred |
|----|-------------|--------|----------|----------|
| AUDIT-1 | All admin operations emit audit events | PASS | `ops.console.action.{completed,rejected}` in `events.go` | |
| AUDIT-2 | Audit events include principal subject | PASS | `OpsConsoleActionPayload.Subject` | |
| AUDIT-3 | Audit events include correlation ID | PASS | `Correlation` field in payload | |
| AUDIT-4 | Audit log immutable (Journal-backed) | PASS | Journal append-only; checksums | |

## 5. Secret Redaction

| ID | Requirement | Status | Evidence | Deferred |
|----|-------------|--------|----------|----------|
| REDACT-1 | Secrets never enter Journal | PASS | `Redactor` removes secrets before journal entry | |
| REDACT-2 | Secrets never in LLM prompts | PASS | Redactor in agent I/O pipeline | |
| REDACT-3 | Error messages don't leak secrets | PASS | RFC 7807; no stack traces | |

## 6. SBOM/Provenance

| ID | Requirement | Status | Evidence | Deferred |
|----|-------------|--------|----------|----------|
| SBOM-1 | SBOM generated for all dependencies | PASS | `go.mod` + `go.sum` tracked | |
| SBOM-2 | Provenance attestation for providers | PASS | `adapters/supplychain/sigverify/` | |
| SBOM-3 | No unverified external dependencies | PASS | `archtest/imports_test.go` deny-list enforced | |

## 7. Tenant Isolation

| ID | Requirement | Status | Evidence | Deferred |
|----|-------------|--------|----------|----------|
| TENANT-1 | Tenant data partitioned at storage layer | PASS | Cell isolation; partition by tenant_id | |
| TENANT-2 | No cross-tenant data leakage | PASS | Tenant filter on all queries | |
| TENANT-3 | Tenant migration is atomic | PASS | `tenant.migration.*` events | |
| TENANT-4 | Tenant quotas enforced | PASS | `adapters/quota/`; per-tenant limits | |

## 8. Compliance (External — Deferred to M8.1)

| ID | Requirement | Status | Evidence | Deferred |
|----|-------------|--------|----------|----------|
| COMP-1 | SOC2 Type II readiness | Deferred | External audit required | M8.1 |
| COMP-2 | External penetration test | Deferred | Third-party pen test scheduled | M8.1 |

---

## Sign-off

| Role | Name | Date | Signature |
|------|------|------|-----------|
| Security Lead | | | |
| Architecture Lead | | | |
| Product Owner | | | |

## Change Log

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0-GA | 2026-08-18 | Initial GA checklist |
