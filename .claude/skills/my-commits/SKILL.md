---
name: my-commits
description: Write git commit messages in my personal style and split working-tree changes into logical commit groups. Every message starts with a prefix (feat, bugfix, refac, test, docs, …) followed by a colon, then a short subject saying what changed, and a short body saying why and how. Use this skill whenever I ask to commit, make commits, write a commit message, "commit my changes", "group these changes", "split this into commits", stage things, or tidy up a messy working tree before pushing — even if I don't mention the format or the word "skill".
---

# My commits

Two jobs: **group** the changes into logical commits, and **write** each message in my format. Propose first, commit only after I say OK.

## Message format

```
<prefix>: <what changed>

<why it was needed — one line>
<how it was done — one or two lines>
```

- **Prefix, colon, space, subject.** No scope in brackets — `bugfix: …`, never `bugfix(auth): …`.
- **Subject = what.** Imperative, lowercase start, no trailing period, aim for ≤ 60 characters (hard cap 72). Someone scanning `git log --oneline` should understand the change from this line alone.
- **Body = why + how.** One blank line after the subject, then 1–3 short lines (wrap at 72). Why comes first because it's the part the diff can't show. How is the approach in plain words, not a file-by-file list.
- **Keep it short.** If the body needs more than 3 lines, the commit is probably doing too much — split it.
- Trivial commits (typo, version bump, formatting) may skip the body.

### Prefixes

Pick the one that tells a reader at a glance what kind of change it is. When two fit, choose the one describing the *main intent*.

| Prefix | Use for |
|---|---|
| `feat` | New user-facing feature or capability |
| `bugfix` | Fixing broken behaviour (normal flow) |
| `hotfix` | Urgent fix going straight to production |
| `refac` | Restructuring code with no behaviour change |
| `perf` | Faster or lighter, same behaviour |
| `security` | Vulnerability fix or hardening |
| `revert` | Undoing an earlier commit (name it in the subject) |
| `test` | Adding/fixing tests only, no app code |
| `docs` | README, docs, code comments |
| `style` | Formatting, lint fixes, whitespace — no logic change |
| `i18n` | Translations, localisation strings |
| `build` | Build system, bundler, Dockerfile |
| `ci` | CI/CD pipelines and workflows |
| `deps` | Adding, bumping or removing dependencies |
| `config` | Env/app config and settings files |
| `migration` | Database schema or data migrations |
| `chore` | Maintenance that fits nowhere else |
| `release` | Version bumps, changelog, tags |
| `init` | First commit / project scaffolding |

Note: it's `refac`, not `refactor`.

### Examples

```
feat: add rate limiting to login endpoint

Stops brute-force password guessing on /login.
Counts attempts per IP in Redis and returns 429 after 5 per minute.
```

```
bugfix: return 404 instead of 500 for unknown order id

Missing orders crashed the handler and paged on-call.
Check for null after the lookup and raise NotFound.
```

```
refac: extract price calculation into PricingService

Same logic was copied in checkout and cart, drifting apart.
Moved it to one service; both callers now use it.
```

```
deps: bump axios to 1.7.4

Picks up the fix for the proxy auth header leak.
```

Bad (and why):
- `Fixed stuff` — no prefix, says nothing.
- `feat(auth): Added login.` — scope, past tense, capital, period.
- `bugfix: fix bug` — subject doesn't say *what*.
- A 15-line body listing every file — that's what the diff is for.

## Grouping changes

One commit = one logical change that makes sense on its own. The guiding question: *if someone reverts this commit, does exactly one idea disappear?*

Keep together:
- **Code and its tests** — `router.ts` + `router.test.ts`, `user_service.py` + `test_user_service.py`. A test belongs with the code it tests, not in a separate `test:` commit. (`test:` is only for test-only work, like adding missing coverage to old code.)
- **Code and the config/types/fixtures it needs** to work.
- **Manifest and lockfile** — `package.json` + `package-lock.json`, `pyproject.toml` + `poetry.lock`.
- **A migration and the model change it supports**, when they're one feature. Use `migration:` only when the migration stands alone.
- **A rename/move and the import updates** it forces.

Split apart:
- Unrelated features or fixes, even in the same file — use `git add -p` to stage hunks separately.
- Pure formatting/lint noise from real changes (`style:` commit, ideally first so later diffs are clean).
- Dependency bumps not required by the feature.
- Docs changes unrelated to a code change in the same set.

Ordering: put commits in an order where each one would build and pass on its own — dependencies and refactors before the features that use them.

Never commit secrets or junk: `.env`, credentials, keys, build output, `node_modules`, OS files (`.DS_Store`). If they show up, flag them and suggest a `.gitignore` entry instead.

## Workflow

1. **Look.** Run `git status`, `git diff`, `git diff --staged`, and check untracked files. Read enough of each diff to understand the intent — don't guess from file names alone.
2. **Group.** Build the commit groups using the rules above.
3. **Propose.** Show the plan, in order:

   ```
   1. feat: add rate limiting to login endpoint
      Files: src/routes/login.ts, src/routes/login.test.ts, src/lib/rateLimit.ts
      Body:
        Stops brute-force password guessing on /login.
        Counts attempts per IP in Redis and returns 429 after 5 per minute.

   2. deps: add ioredis
      Files: package.json, package-lock.json
   ```

   Mention anything skipped (and why) and any file split across commits by hunk. Then ask for OK.
4. **Commit after OK.** For each group, stage only its files — `git add <paths>` (or `git add -p` for partial files); never `git add -A` or `git add .`, since that sweeps in unrelated changes. Commit with `git commit -m "<subject>" -m "<body>"`. If I edit a message or regroup, use my version.
5. **Confirm.** Show `git log --oneline -n <count>` so I can see the result. Don't push unless I ask.

If I only ask for a message (e.g., "write a commit message for this diff"), skip grouping and just give the message in the format above.
