## Summary

Describe the focused user-visible or operator-visible change.

## Reason

Explain the defect, safety gap, or operational need. Link the relevant issue
when one exists.

## Security and compatibility

- Authentication/CIDR impact:
- Probe or external-traffic impact:
- Selector-write impact:
- Config/API compatibility:
- Rollback:

## Verification

List the exact commands and results. Distinguish local, CI, mock, and real-router
evidence.

- [ ] `./tools/verify.ps1`
- [ ] `./tools/smoke-test.ps1`
- [ ] New or changed behavior has focused tests.
- [ ] Embedded web assets match the Vue production build, if `web/` changed.
- [ ] `README.md` and `README.zh-CN.md` remain aligned when shared contracts changed.
- [ ] `docs/architecture.md` and `CHANGELOG.md` were updated when required.
- [ ] Logs, screenshots, and configs contain no secrets or private identifiers.

## Evidence boundary

State what has not been verified, such as a particular Mihomo/OpenClash version,
CPU architecture, real router, strict listener, or long-running workload.
