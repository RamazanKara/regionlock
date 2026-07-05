# Security Policy

## Reporting a vulnerability

Please report suspected vulnerabilities privately via GitHub's
[**Report a vulnerability**](https://github.com/RamazanKara/regionlock/security/advisories/new)
flow (Security → Advisories). Do not open a public issue for security problems.

You can expect an initial acknowledgement within **72 hours** and a remediation
plan or fix timeline within **10 working days**.

## Scope

Regionlock is a policy + evidence tool. Security-relevant areas include:

- The **rule engine** producing a false *pass* (a violation that should have been
  flagged but was not). This is the highest-severity class, since it undermines
  the evidence report.
- The **evidence integrity** layer (digest/signature): any way to alter a report
  without invalidating the digest or signature.
- The **Helm chart / Rego** producing policies that fail open (admit a resource
  they should block).

## What is explicitly out of scope

Regionlock evidences *placement, egress, and key-management controls*. It does
**not** claim to be a cryptographic proof that data never physically left a
region. A report that correctly reflects the enforced controls, but where an
operator has mis-scoped those controls, is not a vulnerability in Regionlock.

## Supported versions

The latest minor release receives security fixes. Pre-1.0, please upgrade to the
newest tag before reporting.
