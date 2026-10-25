# Integration fixtures

These files serve the integration stack of `make e2e` alone: the deployment
file, the password of the test administrator, and `profiles.toml`, the
profiles that point at the stack's fake Prometheus servers. They are public
and weak by design. The server secret is not tracked: `make e2e` copies this
folder to `.cache/e2e/config`, writes a fresh `secret/server-secret` there on
every run, and points `CAPACITYROOM_CONFIG_DIR` at the copy. Never use them for a deployment: a deployment provides its own folder, and its
own database password in `CAPACITYROOM_POSTGRES_PASSWORD`.

The folder mounts at `/opt/capacity-room` with the layout `platform.toml`,
`secret/server-secret` (made by the run), and `credentials/`.

`credentials/` holds the credential file that the profile `e2e-recovery` names.
`e2e-recovery` points at the second fake Prometheus, which answers slowly so
that a report is still in flight while its list is on screen.
