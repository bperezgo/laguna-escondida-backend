<#
.SYNOPSIS
  Runs the backend natively on Windows in EDGE mode with the real print spooler,
  so POST /api/device/print drives the physically-attached thermal printer.

.DESCRIPTION
  The `windows` printer transport talks to the local print spooler (winspool.drv),
  so the edge MUST run as a native Windows process — NOT in the Docker edge
  container (Linux, can't see the USB printer).

  This reuses the docker-compose.sync.yml `edge-db` (localhost:5434), which is
  already migrated and seeded. Stop the docker edge container first so a single
  edge owns that DB and port 8082 is free:

      docker compose -f docker-compose.sync.yml stop edge

  Then run this script in its own terminal (it blocks, serving HTTP). Drive the
  end-to-end print test from a second terminal with scripts\print-e2e-test.ps1.

.PARAMETER Port           HTTP port to serve on (default 8082, matching the docker edge mapping).
.PARAMETER DbPort         Host port of the edge Postgres (default 5434, the docker edge-db).
.PARAMETER PrinterTarget  Windows printer name from `Get-Printer` (default "POS58 Printer").
.PARAMETER PrinterWidthMM 58 or 80 (default 58).
.PARAMETER PrinterCodepage CP850 | CP858 | CP1252 (default CP850).
.PARAMETER PrinterCut     full | partial | none (default partial).
#>
param(
    [int]$Port             = 8082,
    [int]$DbPort           = 5434,
    [string]$PrinterTarget = "POS58 Printer",
    [int]$PrinterWidthMM   = 58,
    [string]$PrinterCodepage = "CP850",
    [string]$PrinterCut    = "partial"
)

$ErrorActionPreference = "Stop"

# --- App identity / secrets (match docker-compose.sync.yml x-app-env so this edge
#     lines up with the existing edge-db data) ---------------------------------
$env:APP_MODE        = "edge"
$env:ORGANIZATION_ID = "test-org"
$env:JWT_SECRET      = "test-jwt-secret"
$env:ADMIN_API_KEY   = "test-admin-key"
$env:NODE_SYNC_KEY   = "test-sync-key"
$env:CLOUD_SYNC_URL  = "http://localhost:8080"   # docker cloud; lets the edge boot past the sync guard

# --- Electronic invoice + object storage: inert dummies (printing never calls them) ---
$env:ELECTRONIC_INVOICE_URL      = "http://invoice.invalid"
$env:ELECTRONIC_INVOICE_USER     = "test"
$env:ELECTRONIC_INVOICE_PASSWORD = "test"
$env:SPACES_REGION = "us-east-1"
$env:SPACES_KEY    = "offline-unused"
$env:SPACES_SECRET = "offline-unused"
$env:SPACES_BUCKET = "offline-unused"

# --- Database: the docker edge-db (already migrated + seeded) -------------------
$env:DB_HOST     = "localhost"
$env:DB_PORT     = "$DbPort"
$env:DB_USER     = "postgres"
$env:DB_PASSWORD = "postgres"
$env:DB_NAME     = "laguna_escondida"
$env:DB_SSLMODE  = "disable"

# --- Printer: the real device via the Windows RAW spooler ----------------------
$env:PRINTER_TRANSPORT = "windows"
$env:PRINTER_TARGET    = $PrinterTarget
$env:PRINTER_WIDTH_MM  = "$PrinterWidthMM"
$env:PRINTER_CODEPAGE  = $PrinterCodepage
$env:PRINTER_CUT       = $PrinterCut

# --- Ticket header identity (shown on the printout) ----------------------------
$env:BUSINESS_NAME    = "Laguna Escondida"
$env:BUSINESS_NIT     = "900.123.456-7"
$env:BUSINESS_ADDRESS = "Vereda La Laguna"
$env:TICKET_FOOTER    = "Gracias por su visita!"

$env:PORT = "$Port"

Write-Host "Starting native EDGE on :$Port  (DB localhost:$DbPort, printer '$PrinterTarget' ${PrinterWidthMM}mm/$PrinterCodepage/$PrinterCut)" -ForegroundColor Cyan
Write-Host "Ticket printing route: POST http://localhost:$Port/api/device/print" -ForegroundColor Cyan
go run ./cmd
