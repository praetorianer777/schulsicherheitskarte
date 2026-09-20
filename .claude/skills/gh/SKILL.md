---
name: gh
description: Use the GitHub CLI (gh) for this repo — find or create issues, create issue branches, open and update PRs, check CI runs and releases. Use before starting any code change (every change needs an issue and an issue branch) and whenever GitHub issues, PRs, Actions or releases are involved.
---

# GitHub CLI (`gh`) for schulsicherheitskarte

Repo: `praetorianer777/schulsicherheitskarte`, default branch `main`.

## Rules

- Every change starts from a GitHub issue and lives on a branch `<type>/<issue>-<slug>`
  (`feat|fix|chore|docs|refactor|test|perf|ci|build|revert`, slug lowercase with dashes).
  `.claude/hooks/branch-guard.sh` blocks edits, commits and pushes anywhere else.
- Never push to `main`, never merge a PR yourself — the user merges.
- `gh` runs non-interactively here: always pass `--title`/`--body` (or `--body-file`),
  never rely on prompts or an editor. Prefer `--json … --jq …` for reading.
- Creating issues, PRs or comments is outward-facing: confirm with the user first
  unless they already asked for it.

## Preflight

```bash
gh auth status          # not logged in → ask the user to run: ! gh auth login
```

## 1. Find or create the issue

```bash
gh issue list --state open --limit 30 --json number,title --jq '.[] | "#\(.number) \(.title)"'
gh issue view 42 --json title,body,state,labels
gh issue create --title "fix: Unfallpunkte fehlen am Radius-Rand" --body-file - <<'BODY'
## Problem

…

## Expected

…
BODY
```

## 2. Create the issue branch

```bash
git fetch origin
gh issue develop 42 --name fix/42-radius-rand --base main --checkout
```

## 3. Commit and push

Commit via `/commit`. Run `./run-tests.sh` before pushing — the branch guard runs it
on every `git push` anyway, so a broken suite blocks the push. Push only the issue
branch:

```bash
git push -u origin HEAD
```

## 4. Pull request

```bash
gh pr create --base main --title "fix: Unfallpunkte am Radius-Rand" --body-file - <<'BODY'
## Summary

…

## Testing

./run-tests.sh

Closes #42
BODY
```

Always put `Closes #<issue>` in the PR body.

## Reading feedback and CI

```bash
gh pr view --comments
gh api repos/{owner}/{repo}/pulls/17/comments --jq '.[] | "\(.path):\(.line) \(.body)"'
gh run list --branch "$(git branch --show-current)"
gh run view <id> --log-failed
```

## Pitfalls

- `gh api` keeps the literal placeholders `{owner}`/`{repo}` — in fish, quote the path.
- Output is paged when attached to a TTY; set `GH_PAGER=cat` if a command hangs.
- `gh pr create` fails if the branch is not pushed yet — push first.
