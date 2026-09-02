# OpenWrt / iStoreOS deployment template

This directory contains a conservative `procd` deployment template for a
router that runs Mihomo through OpenClash. It is not tied to one router model,
address, CIDR, or CPU architecture. Confirm every target value before copying
or starting the service.

The public example uses this security baseline:

- the Mihomo Controller remains on `127.0.0.1:9090`;
- non-loopback API access requires both a generated Bearer token and a narrow
  trusted-CIDR allow-list;
- the Controller secret is passed through the service environment and is not
  copied into the application YAML, database, or logs;
- automatic switching and the dedicated strict-verification listener remain
  disabled; and
- normal operation begins with manual scans and manual selection only.

The bundled init script expects the OpenClash configuration containing the
Controller secret at `/etc/openclash/dukou.yaml`. Verify that path and the
active Controller address on the target. A different Mihomo installation needs
a reviewed init-script adaptation; do not copy a secret into the repository or
the application config as a shortcut.

## Read-only preflight

Before building or changing the router, record:

1. the private LAN or Tailnet address and SSH host-key fingerprint;
2. `ubus call system board`, including the actual CPU architecture;
3. free `/overlay` space and available RAM;
4. `mihomo -v` and the active Controller address;
5. whether `/etc/openclash/dukou.yaml` is the active secret source, without
   printing the secret;
6. the narrow client CIDR that should reach the UI; and
7. whether TCP port `8788` is already in use.

Do not assume that an OpenWrt device is ARM64 because the example build below
uses `arm64`.

## Build

From the project root on Windows PowerShell, substitute the architecture found
during preflight:

```powershell
./tools/build-openwrt.ps1 -GoArch arm64 -OutputPath dist/linux-arm64
```

The build script uses `CGO_ENABLED=0`, prints Go build metadata, and prints the
binary's SHA-256. Stage these files on the router under `/tmp/mss-stage/`:

- `dist/linux-<arch>/mihomo-smart-selector`
- `deploy/openwrt/mihomo-smart-selector.init`
- `deploy/openwrt/config.router.example.yaml`

Verify the binary and deployment-file SHA-256 values on both hosts before
installation. Do not treat a successful copy as proof that the service is
running or using the intended configuration.

## Prepare the configuration

The example listener is `0.0.0.0:8788`, so edit the staged YAML before starting
the service:

- replace `192.168.1.0/24` with the narrow LAN or Tailnet CIDR that contains the
  intended clients;
- keep `api_token_env: MSS_API_TOKEN`;
- keep `allow_unauthenticated_lan: false`;
- confirm the loopback Mihomo Controller address; and
- review probe endpoints, concurrency, candidate limits, and storage path.

The unauthenticated-LAN mode is an explicit high-risk override: every device in
the allowed CIDR can scan and change an authorized Selector without a token.
Do not enable it merely to bypass a login problem, and never combine it with a
broad CIDR or WAN exposure.

## One-time installation

Preserve the current OpenClash state before installing. Hash an override file
only when that file exists; this basic deployment does not modify it.

```sh
/etc/init.d/openclash status
test ! -f /etc/openclash/custom/openclash_custom_overwrite.sh || \
  sha256sum /etc/openclash/custom/openclash_custom_overwrite.sh
```

With the reviewed, verified staged artifacts:

```sh
mkdir -p /opt/mihomo-smart-selector/data /etc/mihomo-smart-selector
chmod 0755 /opt/mihomo-smart-selector /opt/mihomo-smart-selector/data /etc/mihomo-smart-selector
cp /tmp/mss-stage/mihomo-smart-selector /opt/mihomo-smart-selector/mihomo-smart-selector
cp /tmp/mss-stage/config.router.example.yaml /etc/mihomo-smart-selector/config.yaml
cp /tmp/mss-stage/mihomo-smart-selector.init /etc/init.d/mihomo-smart-selector
chmod 0755 /opt/mihomo-smart-selector/mihomo-smart-selector /etc/init.d/mihomo-smart-selector
chmod 0600 /etc/mihomo-smart-selector/config.yaml
```

Edit `/etc/mihomo-smart-selector/config.yaml` now and re-check the listen
address, allowed CIDR, Controller address, and disabled mutation features. Only
then enable and start the service:

```sh
/etc/init.d/mihomo-smart-selector enable
/etc/init.d/mihomo-smart-selector start
```

The init script creates a random token in
`/etc/mihomo-smart-selector/access-token` with the process umask set to `077`.
Keep that file private. Perform the first health check locally without printing
the token:

```sh
token="$(cat /etc/mihomo-smart-selector/access-token)"
curl -fsS -H "Authorization: Bearer $token" \
  http://127.0.0.1:8788/api/v1/health
unset token
```

Open `http://<router-private-address>:8788` from a client inside the configured
CIDR and enter the token retrieved over the trusted SSH session. Do not add a
WAN port forward. Review the visible Probe Profile and its verification scope,
run a filtered scan, and use the explicit selection action. Verify the target
group's current member before accepting the deployment.

## Rollback

Stop and disable the service before removing only its explicitly installed
paths:

```sh
/etc/init.d/mihomo-smart-selector stop
/etc/init.d/mihomo-smart-selector disable
rm -f /etc/init.d/mihomo-smart-selector
rm -rf /opt/mihomo-smart-selector /etc/mihomo-smart-selector
```

This removes the local database and generated API token. It does not change
OpenClash subscription files, strategy groups, the active Mihomo configuration,
or the Controller listener. Restore and restart OpenClash only if a separately
reviewed deployment step changed those files.
