# Security Guidelines

## Existing vulnerabilities found outside the current task

- Do not fix it inline — stay scoped to what was requested.
- Instead, record it in `SECURITY_REVIEW.md` at the project root (create it if it doesn't exist). Each entry should include: location (file:line), description, severity/impact, and a suggested remediation.
- Do not modify the vulnerable code until the user explicitly approves the fix.

## High-risk changes

- Before implementing changes to auth, payments, crypto, or admin/privileged endpoints — even when explicitly requested — summarize the security implications and get explicit confirmation before writing code.
- Treat "high-risk" as: anything that changes who can access what, anything that touches money, anything that touches encryption/hashing/token generation.

## Leaked secret response

If a secret, API key, or credential is found committed (by you or already in history):
- Stop — do not continue the current task until this is addressed.
- Tell the user immediately; do not attempt to silently scrub it yourself.
- Recommend: rotate/revoke the credential at the source (it must be treated as compromised — removing it from the repo does not undo exposure), then purge it from git history.
- Do not purge git history or force-push without explicit approval — that's a destructive, hard-to-reverse operation.

## Input validation & injection

- User input: validate/sanitize at the boundary where it enters the system, not deep inside business logic. Never build queries, shell commands, or file paths via string concatenation of user input — use parameterized queries, safe APIs, or explicit allowlists.
- File uploads: validate type/size server-side (not just via client-side checks or file extension), and avoid using user-supplied filenames directly on the filesystem.
- Third-party API responses: treat as untrusted input — validate shape/type before using, don't assume a response matches its documented schema.
- When rendering user-controlled content in a UI, ensure it's escaped/sanitized for the output context (HTML, SQL, shell, etc.) to prevent injection/XSS.

## Auth & access control

- Default to least privilege: new roles, tokens, or API keys get the minimum scope needed, not broad access "to be safe."
- Enforce authorization checks server-side — never rely on the client (hidden UI elements, disabled buttons) to restrict access.
- Session/token handling: don't log tokens or session identifiers, set secure/httpOnly/short-lived where the framework supports it, invalidate sessions on logout/password change.
- Any change to who can authenticate or what a role can do falls under "High-risk changes" above.

## Dependency vulnerability scanning

- Always ask before adding a new third-party dependency, regardless of how well-known the package is. State why it's needed and whether an existing dependency or the standard library could do the job instead.
- If the project has an audit tool available (`pnpm audit`, `pip-audit`, `cargo audit`, etc.), run it when touching dependency files and report new findings.
- For a known CVE in an existing dependency: prefer patching to the fixed version. If no fix is available, report the CVE, its severity, and exposure in this codebase, and let the user decide between pinning, a workaround, or accepting the risk.

## General practices

- Never hardcode secrets, API keys, or credentials — use environment variables or a secrets manager instead.
- Don't log secrets, tokens, or PII, even at debug level.
- Don't leak internal details (stack traces, file paths, query errors) in user-facing error messages.