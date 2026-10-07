# Integration fixtures

These files serve the integration stack of `make e2e` alone: the deployment
file, the server secret, and the password of the test administrator. They are
public and weak by design. `make e2e` points `KCR_CONFIG_DIR` at this folder.
Never use them for a deployment: a deployment provides its own folder, and its
own database password in `KCR_POSTGRES_PASSWORD`.
