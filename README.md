# Mihomo Smart Selector

`Mihomo Smart Selector` is a small, self-hosted service for choosing a stable
Mihomo selector member for a particular internet service. It is designed for
OpenClash/iStoreOS, but it does not replace OpenClash and does not expose the
Mihomo controller secret to a browser.

The first implemented release provides:

- a Vue 3 dashboard embedded into one Go binary;
- discovery of selector groups, providers, and eligible members through the
  local Mihomo Controller API;
- configurable region classification and node overrides;
- service-aware Probe Profiles that map each selector to a no-credential,
  read-only endpoint instead of one global URL;
- multi-sample scans with median, P95, jitter, success rate, and a transparent
  90-point performance score with an independent score breakdown;
- separate reachability, strict HTTP/body verification, restriction,
  region-verification and transport-scope states, so a 200 response is never
  presented as proof of login, streaming or regional unlock;
- SQLite-backed scan and switch history;
- a deliberate, manual **Select best node** action; and
- an optional, serialized actual-egress check through an explicitly configured
  local Mihomo listener; and
- an opt-in strict-verification route that uses an isolated Mihomo selector and
  local proxy listener, never the business selector being scored.

The service binds to `127.0.0.1:8788` by default. Keep that default until an
authenticated, LAN-restricted access path has been configured.

## Local development

The repository includes no global-toolchain assumption. The development Go
toolchain is intentionally ignored under `.tools/`.

```powershell
$go = "$PWD/.tools/go/bin/go.exe"
& $go mod download
pnpm --dir web install --frozen-lockfile
pnpm --dir web build
& $go test ./...
& $go build -o bin/mihomo-smart-selector.exe ./cmd/mihomo-smart-selector
Copy-Item config.example.yaml config.yaml
$env:MIHOMO_SECRET = '<controller secret>'
./bin/mihomo-smart-selector.exe -config config.yaml
```

Open `http://127.0.0.1:8788` only after the local mock or Mihomo controller is
running. The server will return a clear degraded-health response while Mihomo
is unreachable.

`scanner.probe_profile_overrides` is the safe way to configure a private
service such as Emby without replacing built-in profiles. Strict checks remain
disabled until a dedicated `__SMART_PROBE__`-style selector and loopback proxy
listener have been installed and verified. See
[the architecture and deployment design](docs/architecture.md).

## Router deployment

Deployment is intentionally a separate, verified step. It requires the
router's confirmed **private/Tailscale** address, SSH access, CPU architecture,
Mihomo version, available flash space, and a review of the active OpenClash
configuration. See [the architecture and deployment design](docs/architecture.md).

Do not place a controller secret, provider URL, or a built binary in source
control.
