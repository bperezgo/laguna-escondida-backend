# AGENTS.md

Edge appliance for the Kuenta POS: orchestration, config and provisioning for a single
Windows box. No app source lives here; app artifacts come from verified GitHub Releases.
See `README.md` for the architecture and layout.

## Code style

- Write code that reads on its own: clear names, short lines, small steps.
- Comment only what the code cannot say: a non-obvious *why*, a platform trap, a hard constraint.
- One or two lines per comment. No essays, no restating the code, no history of past attempts.
- No section banners or step numbers unless the file already uses them.
- Don't add comments to code you didn't change.
- Long explanations belong in `docs/` or the commit message, not in the code.

## Conventions

- Install root on the box is `C:\kuentapos`.
- Only Caddy is exposed on the LAN; everything else binds `127.0.0.1`.
- `env\box.env` is the single source of truth for the box's IP, subnet and hostname.
- Per-box files under `env/` (no `.example`), `artifacts/`, `certs/` and data dirs are gitignored.
- Scripts target Windows PowerShell 5.1.
- Pinned third-party versions live in `versions.json`; the app pair comes from the `edge/v*` release (`compat.json` holds `min_cloud`).
