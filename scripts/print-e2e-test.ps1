<#
.SYNOPSIS
  End-to-end test of POST /api/device/print against a running native edge:
  sign in -> create an open bill with one product -> print it on the real device.

.DESCRIPTION
  Run scripts\run-edge-native.ps1 in another terminal first. This script then:
    1. POST /api/auth/signin           -> JWT
    2. POST /api/orders                -> creates an open bill (returns its id)
    3. POST /api/device/print          -> renders + prints the ticket

.PARAMETER ApiUrl     Base URL of the native edge (default http://localhost:8082).
.PARAMETER Username   Login user (default newuser, already seeded in edge-db).
.PARAMETER Password   Login password (default password123).
.PARAMETER ProductId  A product UUID to put on the bill (default a seeded product).
.PARAMETER Copies     Copies to print (default 1).
#>
param(
    [string]$ApiUrl    = "http://localhost:8082",
    [string]$Username  = "newuser",
    [string]$Password  = "password123",
    [string]$ProductId = "019efcb1-5fde-779c-8989-3e457946d5e9",  # Gaseosa Postobon
    [int]$Copies       = 1
)

$ErrorActionPreference = "Stop"

function Show-HttpError($err) {
    if ($err.ErrorDetails -and $err.ErrorDetails.Message) {
        Write-Host "  server said: $($err.ErrorDetails.Message)" -ForegroundColor Yellow
    } else {
        Write-Host "  $($err.Exception.Message)" -ForegroundColor Yellow
    }
}

# 1) Sign in -------------------------------------------------------------------
Write-Host "1) Signing in as $Username ..." -ForegroundColor Cyan
try {
    $signin = Invoke-RestMethod -Method Post -Uri "$ApiUrl/api/auth/signin" `
        -ContentType "application/json" `
        -Body (@{ username = $Username; password = $Password } | ConvertTo-Json)
} catch { Write-Host "Sign-in failed." -ForegroundColor Red; Show-HttpError $_; exit 1 }

$token = $signin.token
if (-not $token) { Write-Host "No token returned." -ForegroundColor Red; exit 1 }
$auth = @{ Authorization = "Bearer $token" }
Write-Host "   token acquired; permissions: $($signin.permissions -join ', ')" -ForegroundColor DarkGray

# 2) Create an open bill with one product --------------------------------------
$openBillId  = [guid]::NewGuid().ToString()
$orderBody = @{
    open_bill_id        = $openBillId
    temporal_identifier = [guid]::NewGuid().ToString()
    descriptor          = "Mesa de prueba"
    products = @(
        @{
            open_bill_product_id = [guid]::NewGuid().ToString()
            product_id           = $ProductId
            quantity             = 2
            notes                = "prueba de impresion"
        }
    )
} | ConvertTo-Json -Depth 5

Write-Host "2) Creating open bill $openBillId ..." -ForegroundColor Cyan
try {
    $order = Invoke-RestMethod -Method Post -Uri "$ApiUrl/api/orders" `
        -Headers $auth -ContentType "application/json" -Body $orderBody
    Write-Host "   created. total_amount=$($order.total_amount) status=$($order.status)" -ForegroundColor DarkGray
} catch { Write-Host "Create order failed." -ForegroundColor Red; Show-HttpError $_; exit 1 }

# 3) Print ---------------------------------------------------------------------
Write-Host "3) Printing ticket ($Copies cop$(if($Copies -ne 1){'ies'}else{'y'})) ..." -ForegroundColor Cyan
$printBody = @{ open_bill_id = $openBillId; copies = $Copies } | ConvertTo-Json
try {
    $print = Invoke-RestMethod -Method Post -Uri "$ApiUrl/api/device/print" `
        -Headers $auth -ContentType "application/json" -Body $printBody
    Write-Host "DONE: printed=$($print.printed) — check the printer." -ForegroundColor Green
} catch { Write-Host "Print failed." -ForegroundColor Red; Show-HttpError $_; exit 1 }
