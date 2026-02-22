# Self-publishing to the Upbound Marketplace

The CI pipeline is already wired up. The `publish-artifacts` job in
`.github/workflows/ci.yml` logs into `xpkg.upbound.io` and runs
`make publish BRANCH_NAME=...`, pushing to
`xpkg.upbound.io/humoflife/provider-kind`. It only needs credentials to
activate.

---

## Step 1 — Upbound account and organization

1. Sign up / log in at <https://cloud.upbound.io>
2. Make sure the **`humoflife`** organization exists (or create it via
   **Organizations → New**)

---

## Step 2 — Create the repository on the Marketplace

```bash
# Install the up CLI if you don't have it
brew install upbound/tap/up

# Log in
up login

# Create the repository (public so anyone can install the provider)
up repository create humoflife/provider-kind --public
```

Alternatively, use the console: **Marketplace → Repositories → Create
Repository** → name it `provider-kind` under the `humoflife` org.

---

## Step 3 — Create a robot with push access

```bash
# Create the robot account
up robot create humoflife/ci-push

# Create a token for it (save the printed username and password)
up robot token create humoflife/ci-push marketplace-push

# Grant push permission to the repository
up repository permission create humoflife/provider-kind \
  --robot humoflife/ci-push \
  --permission write
```

Save the token username and password printed by `token create` — you will
need them in the next step.

---

## Step 4 — Add GitHub repository secrets

In your GitHub repo go to **Settings → Secrets and variables → Actions** and
add:

| Secret name | Value |
|---|---|
| `UPBOUND_MARKETPLACE_PUSH_ROBOT_USR` | robot token username |
| `UPBOUND_MARKETPLACE_PUSH_ROBOT_PSW` | robot token password |

Once these secrets are present, every push to `main` or `release-*` will
automatically build and push a pre-release package (e.g.
`v0.0.0-1.g50bfc81`).

---

## Step 5 — Publish a versioned release

Use the **Tag** workflow (`.github/workflows/tag.yaml`) to cut a versioned
release:

```
GitHub → Actions → Tag → Run workflow
  version: v0.1.0
  message: Initial release
```

This creates a `v0.1.0` git tag. The tag workflow (which delegates to
`upbound/official-providers-ci`'s `provider-tag.yml`) builds and pushes
the versioned package to `xpkg.upbound.io/humoflife/provider-kind:v0.1.0`.

After publishing, users can install the provider with:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-kind
spec:
  package: xpkg.upbound.io/humoflife/provider-kind:v0.1.0
  runtimeConfigRef:
    apiVersion: pkg.crossplane.io/v1beta1
    kind: DeploymentRuntimeConfig
    name: provider-kind
```

as documented in `examples/install.yaml`.
