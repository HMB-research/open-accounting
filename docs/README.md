# Open Accounting Documentation

This directory keeps active product, developer, and operator documentation. Do
not add historical implementation plans or agent-loop notes here; promote
current facts into one of the canonical documents below instead.

## Current State

- [Current Product Limits](./CURRENT_PRODUCT_LIMITS.md) is the concise cap and
  gap summary.
- [Development Status](./DEVELOPMENT_STATUS.md) records verified engineering
  evidence for the current branch.
- [Use Case Coverage](./USE_CASE_COVERAGE.md) maps product use cases to test and
  documentation proof.
- [Architecture](./ARCHITECTURE.md) explains system boundaries, tenant flow, and
  test gates.

## Interfaces

- [API](./API.md) documents HTTP endpoints.
- [CLI](./CLI.md) documents operator commands.
- [Plugins](./PLUGINS.md) documents plugin manifests and runtime behavior.
- [Demo E2E Testing](./demo-e2e-testing.md) documents resettable demo coverage.

## Operations And Integrations

- [Deployment](./DEPLOYMENT.md) covers local, Docker, and production operation.
- [EMTA Integration](./EMTA_INTEGRATION.md) covers Estonian authority export and
  filing boundaries.
- [Merit And SmartAccounts Mapping](./FEATURE_MAPPING_MERIT_SMARTACCOUNTS.md)
  records incumbent-system feature and migration mapping.

## Reference

- [SmartAccounts API PDF](./reference/vendor/smartaccounts/SmartAccounts_API_2026-06-12.pdf)
  is a dated vendor reference captured for migration research.

## Generated Files

Swagger/OpenAPI artifacts in this directory are generated from the backend API:

- `docs.go`
- `swagger.json`
- `swagger.yaml`
