# Edge Downloads API

The box installer (`KuentaSetup-vX.Y.Z.exe`) for the landing site's download page (`landing/src/pages/descargar.astro`). The button there is a plain link to this endpoint.

- **Cloud only.** The edge does not serve this route (`404`).
- **No credential.** Anyone may call it; requests are limited per address.
- **Redirect, not a file.** The cloud answers with a redirect to the installer on the project's GitHub release; GitHub serves the bytes.

## Which installer

The current installer is the one of the newest edge release that the answering cloud can sync with. This is the rule the boxes' updater applies (`edge/scripts/releases.ps1`), so a new box and an updated box land on the same release. A release qualifies when all of these hold:

- its tag is `edge/vX.Y.Z` (`cloud/*` and `landing/*` releases are never considered);
- it is neither a draft nor a prerelease;
- its `manifest.json` can be read and lists `components.installer`, whose `url` is an `https` address under `https://github.com/<EDGE_RELEASES_REPO>/releases/download/`;
- its `min_cloud` is at or below the cloud's own version. A development cloud (version not `vX.Y.Z`, e.g. `dev`) skips this condition.

Versions are compared numerically (`v1.10.0` is newer than `v1.9.0`).

The cloud keeps what it read from GitHub in memory for ten minutes, so a newly published release is offered within ten minutes. When GitHub cannot be reached, it keeps answering with what it last read; a failed read is retried after one minute. A request never waits more than ten seconds on GitHub.

| Setting              | Default                              | Description                                        |
| -------------------- | ------------------------------------ | -------------------------------------------------- |
| `EDGE_RELEASES_REPO` | `bperezgo/laguna-escondida-backend` | `owner/name` of the public repository to read from |

## Endpoints

| Method | Endpoint                     | Auth | Description                              |
| ------ | ---------------------------- | ---- | ---------------------------------------- |
| GET    | `/api/public/edge/installer` | none | Redirect to the current installer        |

---

## Download the Installer

```
GET /api/public/edge/installer
```

No request body or parameters.

### Response (302 Found)

Headers:

| Header          | Value                                                                                                 |
| --------------- | ----------------------------------------------------------------------------------------------------- |
| `Location`      | `https://github.com/bperezgo/laguna-escondida-backend/releases/download/edge/v1.4.0/KuentaSetup-v1.4.0.exe` |
| `Cache-Control` | `no-store`                                                                                            |

A browser follows the redirect and downloads the file. Each redirect is logged with the version served (`Installer download version=v1.4.0`), and nothing about the visitor beyond the usual request log.

### Errors

| Status | When                                                                                         | Body                                  | Headers                                     |
| ------ | -------------------------------------------------------------------------------------------- | ------------------------------------- | ------------------------------------------- |
| 503    | No edge release qualifies, or the release list has never been read successfully by this instance | `{"error": "installer_unavailable"}` | `Retry-After: 300`, `Cache-Control: no-store` |
| 429    | More than 20 requests from one address in ten minutes                                        | `{"error": "Too many requests"}`      | `Retry-After: <seconds left in the window>` |

```json
{
  "error": "installer_unavailable"
}
```
