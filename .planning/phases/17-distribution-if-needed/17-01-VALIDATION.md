# 17-01 Validation — Conditional Distribution Decision Gate

Date: 2026-09-11
Status: complete; no new distribution implementation required from this slice.

## Scope validated

- Phase 17 can stay conditional: existing local build scripts can produce current CLI/Desktop dogfood artifacts after the Phase 15 fix.
- No installer, updater, signing, notification, release publication, tag or GitHub Release mutation was required.
- The active ChatGPT `pjadm` Gateway was not stopped or replaced; validation used a spare loopback Gateway on `127.0.0.1:43138`.

## Commands run

```text
git status
scripts/build-rc.ps1 -Version v1.0.0-rc.local-phase17
.\dist\adm-v1.0.0-rc.local-phase17-windows-amd64.exe -h
.\dist\adm-v1.0.0-rc.local-phase17-windows-amd64.exe --adm-url http://127.0.0.1:43138 gateway start --listen 127.0.0.1:43138 --detach
.\dist\adm-v1.0.0-rc.local-phase17-windows-amd64.exe --adm-url http://127.0.0.1:43138 environment inspect --environment-id env_43a2d0ca74fbc0f1
.\dist\adm-v1.0.0-rc.local-phase17-windows-amd64.exe --adm-url http://127.0.0.1:43138 gateway stop
```

## Artifact outputs

```text
dist/adm-v1.0.0-rc.local-phase17-windows-amd64.exe
dist/adm-desktop-v1.0.0-rc.local-phase17-windows-amd64.exe
dist/SHA256SUMS-v1.0.0-rc.local-phase17.txt
```

Checksums:

```text
0fb7f714dc15ab121c61ea4ca6d99a47019ceb449649e2c97723e0d206611757  adm-v1.0.0-rc.local-phase17-windows-amd64.exe
a70d610b5498b36d3cd178b21c15e241be8d6046dba041ca2c660f4e9a6141bf  adm-desktop-v1.0.0-rc.local-phase17-windows-amd64.exe
```

## Bounded inspect acceptance

The rebuilt local CLI/Gateway served bounded Environment inspection output on the spare port:

```text
fact_count=30
skill_catalog_state=unconfigured
skill_catalog_count=207
skill_selected_count=0
skill_suppressed_disabled_fact_count=207
skill_detail_fact_count=0
mcp_catalog_state=available
mcp_catalog_count=3
mcp_selected_count=3
mcp_detail_fact_count=3
```

This confirms the Phase 15 dogfood fix is present in the local distribution artifacts: unselected disabled Skills are summarized by `skill.catalog` instead of emitted as 207 individual `skill/<id>` disabled facts.

## Follow-up artifact refresh

After the run-observability dogfood mitigation landed at `6b793d3`, a second local artifact refresh was built without changing Phase 17 scope:

```text
scripts/build-rc.ps1 -Version v1.0.0-rc.local-runobs
```

Checksums:

```text
8f900e80705edc8611c65f7d9ec7b0e33b16b47662f1c221dba48c8dc0987de0  adm-v1.0.0-rc.local-runobs-windows-amd64.exe
6171de994ceb9b652ef85a0236a258b5bcccc1b603f267a29586d27e27dac209  adm-desktop-v1.0.0-rc.local-runobs-windows-amd64.exe
```

The active ChatGPT `pjadm` Gateway was not stopped from inside this session. A live probe against that active connection still showed old running-status behavior, so the refreshed artifact must be used by restarting/switching the Gateway before the new `run_status` running-output snapshots are visible in this tool connection.

## Conclusion

Phase 17 does not need installer/updater/signing/notification implementation at this point. Current local artifact refresh plus spare-port Gateway validation is enough to continue dogfood. Keep Phase 17 on standby until another concrete distribution blocker appears.
