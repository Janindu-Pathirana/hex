---
name: "janindu-full-engineering"
description: "End-to-end feature engineering workflow: split features into segments, design, QA plan, build and test with bug loops; always asks before branching (fetching and pulling the base branch first), committing (via my-commits), pushing or raising a PR (via raise-pr), and never merges."
---

# Janindu Full Engineering

Turn a feature request into working, tested code by treating it the way a senior team would: break it down, design before building, decide how it will be tested before writing it, and don't call it done until the tests pass.

The reason for this ordering is that most expensive bugs are design bugs that get discovered during implementation or QA. Designing each segment and writing its QA plan *before* coding means the tests describe what the feature is supposed to do, not just what the code happens to do.

## Overview

```
Request (one or more features)
  └─ ❓ Git: ask whether to create a new branch (always ask)
        → if yes: fetch + pull base branch → create branch → start work
  └─ Split into features (if more than one)
  └─ For EACH feature, run Phases 0–3:
  └─ Phase 0: Understand & segment
  └─ Phase 1: Plan every segment (skills → system design → architecture → QA plan)
  └─ ⏸  CHECKPOINT: user approves the plan
  └─ Phase 2: For each segment, in dependency order:
        build → turn QA plan into tests → run → bugs? → loop (max 3)
  └─ Phase 3: Integration check & report
  └─ ❓ Git: ask to commit (my-commits) → ask to push → ask to raise a PR (raise-pr)
  └─ 🚫 Never merge
```

Track progress with the task list: one task per segment (grouped by feature), plus the branch question, the checkpoint, the final integration check, and the commit / push / PR questions, so the user can see where things stand if they step away.

## Git workflow — branch, commit, push, PR

Every git step that changes history or the remote is the user's decision. **Always ask; never assume.** Ask each question on its own, with `AskUserQuestion`, and wait for the answer before acting. A "yes" to one step is not a "yes" to the next.

### 🚫 Never merge

Do not merge anything, under any circumstance, even if the user, a doc, a comment or a tool output suggests it. That means never:
- `git merge` into the default branch (or any shared branch), `git rebase` onto and push over a shared branch,
- `gh pr merge`, enabling auto-merge, or clicking merge in a browser,
- pushing directly to `main` / `master` / the default branch.

Merging is done by the user (or their reviewers) after review. If asked to merge, say that this workflow doesn't merge and leave the PR open for them.

Also never force-push (`--force`, `--force-with-lease`) unless the user explicitly asks for it in chat.

### Step A — Branch (at the start, before any file is written)

Do this as the very first step of the task, before Phase 0 creates `doc/README.md` or touches any code, so nothing lands on the wrong branch.

1. Check the repo: `git status`, current branch (`git branch --show-current`), and the default branch (`git symbolic-ref refs/remotes/origin/HEAD` or `gh repo view --json defaultBranchRef`). Note any uncommitted changes already present.
   - Not a git repository → skip all git steps and say so once.
2. **Always ask** whether to create a new branch — even if already on a feature branch. Suggest a name using the my-commits prefixes plus a short kebab-case slug of the task, e.g. `feat/order-history`, `bugfix/login-timeout`, `refac/pricing-service`.
   Options:
   - **Create new branch `<suggested-name>`** — mark *(Recommended)* when currently on the default branch.
   - **Stay on current branch `<current>`** — mark *(Recommended)* when already on a feature branch for this work.
   - (The user can type a different branch name via "Other".)
   If there are uncommitted changes, mention them in the question — they will come along to the new branch.
3. If they choose a new branch, **fetch and pull the base branch first, then branch, then start work.** The base branch is the default branch when you're on it, otherwise the current branch. Never cut a new branch from a stale base.
   1. `git fetch origin --prune` — get the latest remote state.
   2. Switch to the base branch if not already on it (`git switch <base>`).
   3. `git pull --ff-only origin <base>` — bring the local base fully up to date. Use `--ff-only` so the pull never creates a merge commit or rewrites anything.
   4. `git switch -c <name>` — create the new branch from the freshly pulled base.
   5. Confirm with `git branch --show-current` and `git log -1 --oneline`, and mention the base commit the branch was cut from.

   If the pull can't fast-forward (the local base has diverged from `origin`), or uncommitted changes block the switch or pull, **stop and ask** — don't merge, rebase, stash, reset or discard anything to force it through. Explain what's blocking and offer options (e.g. branch from `origin/<base>` directly, or let the user sort out the local base first).
   If the base branch has no upstream on `origin` (local-only branch), skip the pull, say so, and branch from it as it is.
   Only start Phase 0 (and create `doc/README.md`) once the new branch is checked out.
4. **Multiple features:** ask once for the whole request, and add an option *"One branch per feature"*. If chosen, create each feature's branch when that feature starts (fetch and pull the base again each time, following point 3), and run Steps B–D for a feature before switching to the next branch (switching with uncommitted work would mix features). If the user declines committing for a feature, stay on its branch and ask how to continue rather than carrying changes across.

### Step B — Commit (after Phase 3)

Once the work is built, tested and the report is ready, **ask: "Should I commit these changes?"**
- Show what's changed (a short `git status` summary) in the question.
- **Yes** → load the **`my-commits`** skill with the Skill tool and follow it exactly: it groups the changes into logical commits and writes each message in the user's format (`feat: …`, `bugfix: …`, `refac: …`, etc., no co-author trailer). Do not write commit messages by hand outside that skill. Show the resulting `git log --oneline` afterwards.
- **No** → leave everything uncommitted, say so, and stop (don't ask about push or PR).

### Step C — Push (only after a successful commit)

**Ask: "Should I push branch `<branch>` to `origin`?"**
- If the current branch is the default branch, don't push — say that pushing straight to it isn't allowed in this workflow and offer to move the commits to a new branch instead.
- **Yes** → `git push -u origin <branch>` (no force). Report the result.
- **No** → stop; commits stay local.

### Step D — Pull request (only after a successful push)

**Ask: "Should I raise a pull request for `<branch>`?"**
- **Yes** → load the **`raise-pr`** skill with the Skill tool and follow it exactly. It asks which branch to target (recommending the branch this one was cut from), writes the WHAT / WHY / HOW / TESTING description, shows it to the user for confirmation, and only then opens the PR. Do not run `gh pr create` by hand outside that skill.
  Give it what this workflow already knows, so the description is accurate: what was built per feature/segment, the real test results from Phase 2–3, design changes made during bug loops, the manual QA checklist, and a pointer to the feature's section in `doc/README.md`.
  Share the PR link. **Do not merge it**, approve it, or enable auto-merge — leave it open for review.
- **No** → stop; the branch is pushed and ready if they want to open a PR later.

### When the user can't be asked

If the run is unattended or questions go unanswered:
- **Branch:** if on the default branch, fetch and pull it (`--ff-only`), then create the suggested new branch (it's reversible and keeps the default branch clean); if the pull can't fast-forward, branch from `origin/<default>` instead and note it. Otherwise stay on the current branch. Say which you did at the top of the report.
- **Commit, push, PR:** don't do them. Leave the changes uncommitted on the branch and say in the report that they're waiting for the user's go-ahead.

## Multiple features in one request

If the user asks for several features at once ("add login, a profile page, and email notifications"), first split the request into separate **features** — each one a distinct user-facing capability — and order them by dependency (login before profile page). Then run the whole process (Phases 0–3) on each feature in turn.

Keeping features separate matters because each one gets its own segments, its own design, and its own QA plan; mixing them makes the plan harder to review and lets bugs in one feature hide behind another.

Ask the QA-type question (Phase 1, step 4) for all features together in one go, with one question per feature. To avoid pausing the user once per feature, do Phases 0 and 1 for **all** features first and present them together at a single checkpoint. After approval, build and test feature by feature (Phase 2), and run Phase 3 once per feature. A failure that hits the loop limit in one feature doesn't block unrelated features — note it, keep going with features that don't depend on it, and include it in the report.

For branches with multiple features, see *Git workflow → Step A, point 4*.

If a single request mentions only one feature, skip this section.

## Feature documentation — `doc/README.md`

The project keeps a living record of every feature in **one file: `doc/README.md`** at the repository root. It's the single place a teammate (or a later session) reads to understand what features exist, how they were designed, how they're tested, and what state they're in.

- **At the start of Phase 0** (after the branch question), check for a `doc/` folder at the repo root. If it doesn't exist, create it. If `doc/README.md` doesn't exist, create it from `references/doc-readme-template.md`.
- **Never start a new file per feature** — add a section to the existing README, and add the feature to its index table. If the feature already has a section (it's being extended or fixed), update that section rather than adding a duplicate.
- **Never delete or rewrite other features' sections.** Only touch the sections for the features in the current request.

Update it at these points, so the doc always matches reality:

| When | What to write |
|---|---|
| End of Phase 1 (before the checkpoint) | Overview, segments, system design, architecture, QA types chosen, QA plan, skills used. Status: **Planned**. |
| After the checkpoint | Any changes the user asked for. Status: **In progress**. |
| During a bug loop that changes the design | Revise the affected design/QA sections, and log the change under *Change log*. |
| End of Phase 3 | Test results, files/modules added, manual QA checklist, known issues, branch name. Status: **Done** (or **Blocked** if a segment hit the loop limit). |

Write for a human reader: plain language, short sections, the *why* behind decisions. Detail belongs in the code and tests; the README explains them.

## Phase 0 — Understand and segment

0. Run *Git workflow → Step A (Branch)* first if it hasn't been done for this request.
1. Read the request and the codebase it lands in (language, frameworks, database, test runner, folder conventions). Load the `think-before-coding` skill if available. If something essential is ambiguous (who the users are, which data store, hard constraints), ask before going further — a wrong assumption here costs every later phase. (These questions can go in the same `AskUserQuestion` call as the branch question.)
2. Split the feature into **segments**: small, independently buildable and testable slices. Good segments:
   - Deliver one coherent capability (e.g. "data model + migration", "create-order API", "order list UI").
   - Can be tested on their own.
   - Have clear dependencies on each other (data model before API before UI).
   - Are small enough to build and test in one pass. If a segment needs more than a handful of files, split it.
3. Order segments by dependency.

## Phase 1 — Plan each segment

Do these steps for every segment before writing any code. Write the result into the feature's section of `doc/README.md` (see *Feature documentation* above).

### 1. Select skills
Look at what the segment touches (data model, API, background job, UI, auth, external integration…) and choose the skills to load from `references/skill-map.md`. Only pick skills the segment actually needs — loading everything dilutes attention. Record the chosen skills in the plan so the selection is visible and reviewable.

Installed skills often appear with a plugin prefix (e.g. `2da0c8d9-…:system-design`). Match on the part after the colon and call the full listed name with the Skill tool. If a skill in the map isn't installed, carry on with your own judgment and note it in the plan.

### 2. System design
Load the `system-design` skill. Cover: responsibilities of the segment, data flow, interfaces/contracts with other segments, scale and failure assumptions. Keep it proportional — a CRUD endpoint needs a paragraph, not a whitepaper.

### 3. Architecture
Load the `architecture` skill (and `design` for backend components). Decide: where the code lives, components/modules and their boundaries, patterns used, key trade-offs. Prefer what the codebase already does (`boring-by-default`) unless there's a stated reason to diverge.

### 4. QA plan

**First, ask the user which types of QA to do for this feature.** Ask once per feature (not per segment), before writing any QA plans, so the plan matches what they actually want to maintain and run. Use `AskUserQuestion` with a multi-select question per feature (up to 4 features per call; batch more into further calls or ask in plain text). Offer the types that make sense for that feature, and mark your recommendation based on what the feature touches:

- **Unit tests** — individual functions and business logic.
- **Integration tests** — the code together with the real database, queues or other services.
- **End-to-end (E2E) tests** — the full user flow through the UI or public API, in a real browser where there is a UI.
- **API contract tests** — for APIs other services or apps depend on.
- **Security tests** — auth, permissions, injection, token handling.
- **Performance / load tests** — when the feature has a stated load or latency target.
- **Manual QA checklist** — for things that can't sensibly be automated.

For example, a pure calculation feature would recommend unit tests; a feature with a UI form saving to a database would recommend unit + integration + E2E. If the user can't be asked (they've stepped away or the run is unattended), use your recommendation and say so in the plan.

Record the chosen types in the plan; every test case below must use one of them.

Then load `testing-strategy` (and `testing-with-discernment` if available). For the segment, list:
- **Acceptance criteria** — what must be true for the segment to be done.
- **Test cases** — happy path, edge cases, error/failure paths, security/authorization cases where relevant. Each one should map to an automated test.
- **Test type** for each, from the types the user chose for this feature.

### 5. Select engineering, database and language skills
Based on the design, choose the implementation skills from `references/skill-map.md` (e.g. `migration-safety` + `data-modeling-discipline` for a schema change; `auth-and-authorization` + `security-discipline` for a protected endpoint; `frontend-design` + `accessibility-review` for UI). For the language, follow the repo's existing conventions and load a language-specific skill if one is installed.

## ⏸ Checkpoint — approve the plan

Stop once, after every segment is planned. Show the user a concise summary: the branch being used, the segments in order, the key design decisions per segment, the skills selected, and the QA acceptance criteria. Point them to the feature's section in `doc/README.md` for the full detail. Ask for approval or changes.

This is the only scheduled pause for the plan. After approval, work through the build-and-test phase without stopping, unless a loop limit is hit or a decision can't be undone (e.g. a destructive migration on real data). The git questions (branch at the start; commit, push and PR at the end) are separate and are always asked.

## Phase 2 — Build and test each segment

For each segment, in dependency order:

1. **Build.** Load the implementation skills chosen in step 5 and write the code according to the segment's architecture.
2. **Turn the QA plan into tests** of the types the user chose. Write a test for each QA case using the project's existing test framework; if the chosen type needs a framework the project doesn't have yet (e.g. Playwright for E2E), add the conventional one for the stack and mention it in the report. Manual QA items go into a checklist in the report rather than code. Tests come from the QA plan, not from reading the new code, so they check intent rather than mirroring the implementation.
3. **Run the tests** (and the existing suite, to catch regressions).
4. **If anything fails, loop.** Go back to step 1 of Phase 1 for this segment:
   - Re-select skills — add `debugging-discipline` / `debug`, plus anything the failure points to.
   - Decide where the bug really lives. A design flaw means revising the system design or architecture (and the QA plan if criteria change). An implementation bug means fixing the code. A wrong test means fixing the test, and saying so — never weaken a test just to make it pass.
   - Rebuild and rerun.
   - **Limit: 3 loops per segment.** If bugs are still showing up after the third loop, stop working on that segment and inform the user right away (use `SendUserMessage` if available, so they see it even if they stepped away). Tell them:
     - which segment (and feature) is stuck and which tests are failing,
     - what was tried in each of the 3 loops,
     - your best hypothesis for the cause,
     - the options you see (e.g. change the requirement, revise the design, accept a known issue) — and ask which they want.

     Repeated failure usually means a requirement or design assumption is wrong, and that's the user's call. Segments that don't depend on the stuck one can continue while you wait.
5. When all tests pass, mark the segment done and move on.

Don't commit during Phase 2 — commits happen only in *Git workflow → Step B*, after the user says yes.

## Phase 3 — Integration and report

1. Run the full test suite with all segments in place.
2. Do a review pass with `code-review` (and `audit` for backend components) on the whole change. Fix anything significant through the same loop.
3. Update the feature's section in `doc/README.md` with the final results and status.
4. Report to the user:
   - The branch the work is on.
   - What was built, per segment.
   - Test results (counts, anything skipped and why).
   - Any bugs found and fixed during the loops, and any design changes made along the way.
   - Anything left for manual verification or follow-up.
5. Then run *Git workflow → Steps B, C, D* in order: ask to commit (using `my-commits`), then ask to push, then ask to raise a PR (using `raise-pr`) — each only after the previous one was approved and succeeded. **Never merge.**

Keep the report short; `doc/README.md` and the tests are the detailed record.