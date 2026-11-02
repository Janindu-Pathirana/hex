# Skill map

Which installed skill to load at each step. Names are shown without their plugin prefix; match on the part after the colon in the available-skills list. Pick only what the segment needs.

## Phase 0 — Understand & segment
| Need | Skill |
|---|---|
| Clarify the problem before coding | `think-before-coding` |

## Phase 1 — Planning
| Step | Skill | When |
|---|---|---|
| System design | `system-design` | Every segment |
| Architecture | `architecture` | Every segment |
| Backend component design | `design` | Endpoints, workers, consumers, crons, CLIs, integrations |
| Keep it simple | `boring-by-default` | Choosing patterns / libraries |
| QA plan | `testing-strategy` | Every segment |
| Deciding what's worth testing | `testing-with-discernment` | Every segment, if installed |
| API contract | `oas-generator`, `openapi-parser` | New or changed HTTP APIs |

## QA types → skills
Use for the QA types the user chose for the feature.

| QA type | Skill |
|---|---|
| Unit / integration | `testing-strategy`, `testing-with-discernment` |
| Integration on Azure SQL | `azuresql-db-testing` |
| E2E (web UI) | `verify` (qa-my-app) to run flows in a real browser; Playwright in the repo for committed tests |
| E2E (mobile) | `run-automation-suite`, `create-test-run` (Kobiton) |
| API contract | `pactflow`, `pact-coverage`, `drift-testing` |
| Security | `security-discipline`, `auth-and-authorization` |
| Performance / load | `performance-and-scaling` |
| Manual QA checklist | `qa-quest` for a guided session together |

## Phase 1 step 5 / Phase 2 — Implementation

### Engineering discipline
| Concern | Skill |
|---|---|
| Error handling | `error-handling-as-design` |
| Retries, side effects, duplicate requests | `idempotency-and-side-effects` |
| Auth & permissions | `auth-and-authorization`, `auth0` (if using Auth0) |
| Security | `security-discipline` |
| Logging / metrics / tracing | `observability-by-default` |
| Performance & scale | `performance-and-scaling` |

### Database
| Concern | Skill |
|---|---|
| Schema / data model | `data-modeling-discipline` |
| Writing queries | `query-discipline`, `explain-this-query` |
| ORM loops | `hunt-n-plus-one` |
| Migrations | `migration-safety`, `review-migration` |
| Azure SQL specifics | `azuresql-db-schema-migration`, `azuresql-db-connections`, `azuresql-db-auth`, `azuresql-db-seed`, `azuresql-db-testing` (and other `azuresql-db-*` as relevant) |

### Frontend
| Concern | Skill |
|---|---|
| UI design & build | `frontend-design` |
| Accessibility | `accessibility-review` |
| UI copy | `ux-copy` |

### Cloud / deploy (only if the segment includes infra)
| Concern | Skill |
|---|---|
| Azure Functions | `functions-scaffold` |
| App Service | `app-service-deploy` |
| AKS | `aks-manifest` |
| Bicep IaC | `bicep-template` |

### Language
No language-specific skills are in the standard set. Follow the repo's existing conventions (linters, formatters, project layout). If a language skill is installed (e.g. one for TypeScript, Python, Go), load it here.

## Phase 2 — Bug loop
| Need | Skill |
|---|---|
| Root-causing a failure | `debugging-discipline`, `debug` |
| Hunting for further bugs | `qa-bug-hunter` |

## Phase 3 — Review
| Need | Skill |
|---|---|
| Whole-change review | `code-review` |
| Backend component audit | `audit` |
