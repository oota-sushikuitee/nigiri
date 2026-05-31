# Security Policy

## Supported Versions

Nigiri uses a rolling-release model. Security fixes ship only on the latest
tagged release on `main`. Pin a specific version with `go install ...@vX.Y.Z` or
the `Okabe-Junya/tap/nigiri` Homebrew formula if reproducible builds matter.

| Version           | Supported |
|-------------------|-----------|
| Latest tag        | Yes       |
| Older tags        | No        |
| `main` (untagged) | No        |

## Reporting a Vulnerability

Please **do not** open a public GitHub issue for security reports.

Instead, use **GitHub Security Advisories** to disclose privately:

1. Open <https://github.com/oota-sushikuitee/nigiri/security/advisories/new>
2. Describe the issue, the affected version(s), and minimal reproduction steps
3. Include any proof-of-concept code or commands in the advisory body, not in
   public comments

If GitHub Security Advisories is unavailable, fall back to emailing
`junya.okabe.ig@gmail.com` with subject `[nigiri security]`.

### What to expect

| Stage                       | Target window                |
|-----------------------------|------------------------------|
| Acknowledgement             | Within 72 hours              |
| Initial assessment          | Within 7 days                |
| Coordinated disclosure date | Within 90 days of report     |

Critical issues (remote code execution, credential exfiltration, supply-chain
compromise) are prioritized over the normal queue. A CVE will be requested for
any issue that warrants public coordination, and the fix will ship in a tagged
release with a security advisory attached.

## Scope

In scope:

- The `nigiri` binary itself (build, run, storage paths, configuration parsing)
- The `Okabe-Junya/tap/nigiri` Homebrew formula and its build provenance
- Any third-party dependency pulled by `go.mod` if the vulnerability is
  exploitable through `nigiri`'s normal usage

Out of scope:

- The behavior of upstream projects that `nigiri` builds (report those to their
  own maintainers)
- Vulnerabilities in `git` / `go` / OS toolchain that `nigiri` only invokes
- DoS via large or malformed local config files (treated as bugs, not security)

## Disclosure

Once a fix is released, the security advisory will be made public and credit
the reporter unless they request anonymity.
