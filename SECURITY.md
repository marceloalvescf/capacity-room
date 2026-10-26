# Security policy

## Supported versions

Only the latest minor version receives fixes. A fix ships as a new patch or
minor version, with its image and chart, and is named under `Security` in the
[changelog](CHANGELOG.md).

## Reporting a vulnerability

Report it privately through GitHub's private vulnerability reporting: open
the repository's **Security** tab and choose **Report a vulnerability**. Do not
open a public issue or pull request.

Say which version you ran, how it was deployed (the command, Docker Compose,
or the Helm chart), and how to reproduce the problem. Leave out any real
token, password, or metric-source address.

You will get an answer through the advisory. Once a fix is released, the
advisory is published with credit to you, unless you ask otherwise.

## Scope

[docs/security.md](docs/security.md) holds the threat model, the controls,
and each security review. The e2e fixtures under `deploy/platform/` hold weak
passwords and credentials on purpose; they serve the test stack alone and are
not a finding.
