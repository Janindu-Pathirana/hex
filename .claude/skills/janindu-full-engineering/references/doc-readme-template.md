# Template for `doc/README.md`

Use this to create `doc/README.md` when it doesn't exist. When it does exist, copy only the **feature section** block below for each new feature and add a row to the index table.

---

```markdown
# Feature Documentation

This file documents every feature in this project: what it does, how it was designed, how it is tested, and its current status. Each feature has its own section below.

## Features

| Feature | Status | Last updated |
|---|---|---|
| [Feature name](#feature-name) | Planned / In progress / Done / Blocked | YYYY-MM-DD |

---

<!-- One section per feature. Copy from here for each new feature. -->

## Feature name

**Status:** Planned | In progress | Done | Blocked
**Last updated:** YYYY-MM-DD

### Overview
What the feature does and who it is for, in two or three sentences. The original request, briefly.

### Segments
| # | Segment | Depends on | Status |
|---|---|---|---|
| 1 | e.g. Data model + migration | — | Planned |
| 2 | e.g. Share API | 1 | Planned |

### System design
Responsibilities, data flow, interfaces with other parts of the system, scale and failure assumptions. Per segment where they differ.

### Architecture
Where the code lives, main modules/components, patterns used, key trade-offs and why.

### QA
**Test types chosen:** e.g. Unit, Integration, E2E

| Segment | Acceptance criteria | Test cases | Type |
|---|---|---|---|
| 1 | … | … | Unit |

**Manual QA checklist** (if any)
- [ ] …

### Skills used
Planning: … · Implementation: … · Testing: …

### Implementation
Files and modules added or changed (filled in after build).

### Test results
Pass/fail counts per test type, anything skipped and why (filled in after Phase 3).

### Known issues
Open bugs, limitations, anything blocked at the loop limit.

### Change log
- YYYY-MM-DD — Planned.
- YYYY-MM-DD — Design changed: … (reason).
- YYYY-MM-DD — Done.
```
