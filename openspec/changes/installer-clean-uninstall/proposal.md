## Why

Uninstalling Kuenta always keeps the box's data and configuration, so the next install is a repair of the same box. An owner who wants to start over (new network, new box, a test reinstall) can't do it with the installer. They have to delete `C:\kuentapos`, services, the task, the firewall rule, the uninstall entry, machine variables and a trusted certificate by hand. The owner hit this on the first `edge/v1.3.1` reinstall.

## What Changes

- The uninstaller asks once, after the owner confirms the uninstall: `¿Desea borrar también los datos de esta caja (base de datos, configuración, copias de seguridad y certificados)?`, warning that what the box hasn't synced to Kuenta is lost. **No** is the default.
  - **No:** today's behaviour. Data is kept, and the next install repairs the box.
  - **Sí:** everything the appliance put on the PC is removed: `C:\kuentapos` with the database, `env\`, backups and certificates, the box's root certificate in the Windows trust store, and the machine variables the scripts set. The next install is a fresh one.
- A silent uninstall keeps the data, unless it's started with `/PURGE`.
- The last screen says which of the two happened.
- The box's enrollment in the cloud is not touched. A fresh install afterwards gets the existing "La organización ya tiene una caja… ¿reemplazarla?" question, and the owner answers yes.

## Non-goals

- Releasing the box in the cloud during uninstall. That needs a new cloud endpoint; the replace question covers it.
- Checking whether the box has unsynced data before deleting. The warning names the risk.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `edge-installer`: "Uninstalling keeps the restaurant's data" becomes "keeps it unless the owner chooses to remove it".

## Impact

- No backend, frontend, API or sync change. Ships in an `edge/vX.Y.Z` release (planned `edge/v1.3.2`).
- CI covers it: the installer test adds a full-removal uninstall followed by a fresh install.
- Trust boundary: none crossed. Only an elevated administrator runs the uninstaller. The trust-store removal is limited to the box's own root certificate, matched by thumbprint.
- Touches:
  - `edge/installer/kuenta.iss` (uninstall code only)
  - `edge/scripts/uninstall.ps1`
  - `edge/installer/test/run-installer-tests.ps1`
  - `edge/README.md`, `edge/docs/DEPLOYMENT_RUNBOOK.md` (the uninstall section)
- Depends on: none. Shares `kuenta.iss` and `run-installer-tests.ps1` with `installer-busy-page`, so the two merge one after the other.
