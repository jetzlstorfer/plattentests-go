# Easy Auth for Azure Container Apps

This document explains how Azure AD Easy Auth is integrated into the `plattentests-go`
application and what manual steps you need to perform once when setting up a new environment.

---

## How it works

[Azure Container Apps authentication (Easy Auth)](https://learn.microsoft.com/azure/container-apps/authentication)
is a built-in authentication layer managed by the Azure platform. When enabled it:

1. Intercepts every incoming request before it reaches the application container.
2. Validates the session cookie / bearer token against the configured identity provider (Azure AD).
3. If the request is authenticated, forwards the user's identity to the container via HTTP headers:
   | Header | Content |
   |--------|---------|
   | `X-MS-CLIENT-PRINCIPAL-NAME` | User's display name / UPN |
   | `X-MS-CLIENT-PRINCIPAL-ID`   | User's object ID (GUID) |
   | `X-MS-CLIENT-PRINCIPAL`      | Base64-encoded JSON of all claims |

The application reads the `X-MS-CLIENT-PRINCIPAL-NAME` header in the `/createPlaylist` handler.
If the header is present the request is considered authenticated. If it is absent (Easy Auth not
active, or the session cookie is missing), the handler redirects the browser to
`/.auth/login/aad` and returns the user to the original `/createPlaylist` URL after sign-in.

---

## One-time manual setup

### 1. Create an Azure AD App Registration

These steps are performed once per environment (dev / prod) in the Azure Portal or Azure CLI.

#### Option A – Azure Portal

1. Open **Azure Active Directory → App registrations → New registration**.
2. **Name**: e.g. `plattentests-auth`
3. **Supported account types**: *Accounts in this organizational directory only* (single tenant) or *Any Azure AD directory* depending on your audience.
4. **Redirect URI** (Web): `https://<your-container-app-fqdn>/.auth/login/aad/callback`
   - The FQDN is shown on the Container App overview page (e.g. `plattentests.nicegrass-abc123.westeurope.azurecontainerapps.io`).
5. Click **Register**.
6. Note the **Application (client) ID** — you will need it shortly.
7. Go to **Certificates & secrets → New client secret**, create a secret, and note the **Value** (only shown once).

#### Option B – Azure CLI

```bash
# Replace <FQDN> with your Container App's fully-qualified domain name
FQDN="plattentests.nicegrass-abc123.westeurope.azurecontainerapps.io"

az ad app create \
  --display-name "plattentests-auth" \
  --sign-in-audience AzureADMyOrg \
  --web-redirect-uris "https://${FQDN}/.auth/login/aad/callback"

# Note the appId from the output, then create a client secret
APP_ID="<appId from previous command>"
az ad app credential reset --id "$APP_ID" --append
# Note the password (client secret) from the output
```

---

### 2. Store secrets in GitHub

Add the following repository secrets in **Settings → Secrets and variables → Actions**:

| Secret name | Value |
|-------------|-------|
| `PLATTENTESTS_AAD_CLIENT_ID` | Application (client) ID from step 1 |
| `PLATTENTESTS_AAD_CLIENT_SECRET` | Client secret value from step 1 |

The existing `PLATTENTESTS_AZURE_TENANT_ID` secret is reused for the tenant ID.

Once these two secrets are present the *Configure Easy Auth on containerapp* workflow step will
automatically run on every deployment and keep the Container App authentication settings in sync.

---

### 3. Verify Easy Auth is active

After the next successful deployment run:

```bash
az containerapp auth show \
  --name plattentests \
  --resource-group aca-plattentests \
  --query "{enabled: globalValidation.unauthenticatedClientAction, provider: identityProviders.azureActiveDirectory.enabled}"
```

Expected output:
```json
{
  "enabled": "AllowAnonymous",
  "provider": true
}
```

Open `https://<FQDN>/createPlaylist` in a browser. The application should redirect you to
`/.auth/login/aad`, Azure should send you through the Microsoft login page, and after sign-in you
should be returned to `/createPlaylist`.

Do not validate Easy Auth by opening `/.auth/login/aad/callback` directly in the browser address
bar. That endpoint only works when Microsoft Entra redirects back with the expected auth state and
parameters; direct access commonly returns HTTP 401.

---

## Authentication flow in production

```
Browser → Azure Container Apps ingress
            │
            ├─ No session cookie → redirect to /.auth/login/aad
            │                       → Microsoft login → callback → set cookie
            │
            └─ Valid session cookie → forward request + X-MS-CLIENT-PRINCIPAL-NAME header
                                        → plattentests-go app handles /createPlaylist
```

The public `/` route remains accessible to anonymous users because the Container App is configured
with `--unauthenticated-client-action AllowAnonymous`. Only `/createPlaylist` enforces
authentication — the platform forwards the identity header only for authenticated sessions, and
the application redirects anonymous browser requests into the Easy Auth login flow when the header
is absent.

---

## Local development

To test the `/createPlaylist` endpoint locally you need to simulate the Easy Auth header.
You can use `curl` or any HTTP client:

```bash
curl -H "X-MS-CLIENT-PRINCIPAL-NAME: you@example.com" http://localhost:8081/createPlaylist
```

> **Note**: locally the header is not validated cryptographically — it is only checked for
> presence. Do not expose the application directly to the internet without Easy Auth enabled on
> the Container App.

---

## Revoking access

To disable Easy Auth without deleting the App Registration:

```bash
az containerapp auth update \
  --name plattentests \
  --resource-group aca-plattentests \
  --enabled false
```

Remove the `PLATTENTESTS_AAD_CLIENT_ID` and `PLATTENTESTS_AAD_CLIENT_SECRET` GitHub secrets to
prevent the workflow from re-enabling it.

---

## Troubleshooting callback 401

If `/.auth/login/aad/callback` returns 401 after an actual login flow (not direct browsing),
check these items:

1. App Registration has redirect URI exactly set to
  `https://<FQDN>/.auth/login/aad/callback`.
2. App Registration has ID token issuance enabled:
  ```bash
  az ad app update --id <APP_ID> --enable-id-token-issuance true
  ```
3. The configured tenant matches `PLATTENTESTS_AZURE_TENANT_ID`.
4. Easy Auth provider config contains `clientId`, `tenantId`, and issuer pointing to that tenant:
  ```bash
  az containerapp auth microsoft show \
    --name plattentests \
    --resource-group aca-plattentests
  ```
