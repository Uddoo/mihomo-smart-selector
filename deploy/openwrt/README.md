# OpenWrt / iStoreOS deployment package

This package targets the verified NanoPi R5S LTS (`linux/arm64`) environment.
It is intentionally conservative:

- the UI binds to `0.0.0.0:8788` only for the verified `192.168.50.0/24` LAN;
- in the approved no-password LAN mode, every `/api/` route still requires
  that CIDR, while a token remains available as a recovery option if the mode
  is turned off;
- the existing Mihomo Controller remains untouched at `127.0.0.1:9090`;
- the Controller secret is read at startup from the existing
  `/etc/openclash/dukou.yaml`, but is never copied into this repository,
  application YAML, database, or logs;
- automatic switching and the dedicated `__SMART_PROBE__` listener remain
  disabled; and
- normal operation begins with manual scans and manual selection only.

## Build

From the project root on Windows PowerShell:

```powershell
./tools/build-openwrt.ps1 -GoArch arm64 -OutputPath dist/linux-arm64
```

Copy these exact files to the router's `/tmp/mss-stage/` directory:

- `dist/linux-arm64/mihomo-smart-selector`
- `deploy/openwrt/mihomo-smart-selector.init`
- `deploy/openwrt/config.router.example.yaml`

Verify SHA-256 on both hosts before installation.

## One-time installation

The operator should first preserve the current state:

```sh
sha256sum /etc/openclash/custom/openclash_custom_overwrite.sh
/etc/init.d/openclash status
```

Then, with the verified staged artifacts:

```sh
mkdir -p /opt/mihomo-smart-selector/data /etc/mihomo-smart-selector
chmod 0755 /opt/mihomo-smart-selector /opt/mihomo-smart-selector/data /etc/mihomo-smart-selector
cp /tmp/mss-stage/mihomo-smart-selector /opt/mihomo-smart-selector/mihomo-smart-selector
cp /tmp/mss-stage/config.router.example.yaml /etc/mihomo-smart-selector/config.yaml
cp /tmp/mss-stage/mihomo-smart-selector.init /etc/init.d/mihomo-smart-selector
chmod 0755 /opt/mihomo-smart-selector/mihomo-smart-selector /etc/init.d/mihomo-smart-selector
chmod 0600 /etc/mihomo-smart-selector/config.yaml
/etc/init.d/mihomo-smart-selector enable
/etc/init.d/mihomo-smart-selector start
```

The first runtime test stays local to the router:

```sh
curl -fsS http://127.0.0.1:8788/api/v1/health
```

Open `http://192.168.50.2:8788` from that trusted LAN. No token is requested
while `allow_unauthenticated_lan: true` and the source remains inside
`192.168.50.0/24`; do not add a WAN forward. Choose a target group, review the
visible Probe Profile and its verification scope, run a filtered scan, and use
the explicit selection action. Verify the target group member with the local
Controller API before treating the result as accepted.

## Rollback

```sh
/etc/init.d/mihomo-smart-selector stop
/etc/init.d/mihomo-smart-selector disable
rm -f /etc/init.d/mihomo-smart-selector
rm -rf /opt/mihomo-smart-selector /etc/mihomo-smart-selector
```

This does not change OpenClash subscription files, strategy groups, the active
Mihomo configuration, or its Controller listener.
