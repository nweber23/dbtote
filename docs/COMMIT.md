# Commit Guidelines

## Format

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<optional scope>): <subject>
```

Common types: `feat`, `fix`, `refactor`, `chore`, `docs`, `test`, `style`, `perf`, `build`, `ci`.

- No hard character limit on the subject line — just keep it a single, clear line.
- Use imperative mood ("add", "fix", not "added", "fixes").

## Body

A body is optional for trivial, self-explanatory commits (typo fixes, small tweaks).

For anything non-trivial, add a body that explains **why**, not what — the diff already shows what changed.

## Trailers

- **Never add a `Co-Authored-By` trailer, under any circumstances.** This is a hard rule — do not attribute commits to Claude or any other agent/tool.
- No other trailers are required by default.