[CmdletBinding()]
param(
    [Parameter(Mandatory = $false)]
    [ValidateSet(
        "Start",
        "ViewDetails",
        "Buy",
        "ConfirmBuy",
        "AnotherDeal",
        "ContactSupport",
        "PayPalReturn",
        "PayPalCancel"
    )]
    [string]$Action = "Start",

    [Parameter(Mandatory = $false)]
    [string]$BaseUrl,

    [Parameter(Mandatory = $false)]
    [string]$TelegramSecret,

    [Parameter(Mandatory = $false)]
    [long]$TelegramUserId = 123456789,

    [Parameter(Mandatory = $false)]
    [long]$ChatId = 123456789,

    [Parameter(Mandatory = $false)]
    [string]$FirstName = "Dev",

    [Parameter(Mandatory = $false)]
    [string]$Username = "devuser",

    [Parameter(Mandatory = $false)]
    [string]$LanguageCode = "en",

    [Parameter(Mandatory = $false)]
    [long]$ListingId,

    [Parameter(Mandatory = $false)]
    [long]$CurrentListingId,

    [Parameter(Mandatory = $false)]
    [string]$OrderNumber = "TEST-ORDER",

    [Parameter(Mandatory = $false)]
    [switch]$ShowPayload
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Get-RepoRoot {
    $scriptDir = Split-Path -Parent $PSCommandPath
    return Split-Path -Parent $scriptDir
}

function Import-DotEnv {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path
    )

    if (-not (Test-Path -LiteralPath $Path)) {
        return
    }

    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if ($trimmed.Length -eq 0 -or $trimmed.StartsWith("#")) {
            continue
        }

        if ($trimmed.StartsWith("export ")) {
            $trimmed = $trimmed.Substring(7).Trim()
        }

        $parts = $trimmed.Split("=", 2)
        if ($parts.Count -ne 2) {
            continue
        }

        $key = $parts[0].Trim()
        $value = $parts[1].Trim()
        if ($key.Length -eq 0) {
            continue
        }

        if ($value.Length -ge 2) {
            if (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'"))) {
                $value = $value.Substring(1, $value.Length - 2)
            }
        }

        $existingValue = [Environment]::GetEnvironmentVariable($key, "Process")
        if ([string]::IsNullOrWhiteSpace($existingValue)) {
            Set-Item -LiteralPath "Env:$key" -Value $value
        }
    }
}

function Resolve-BaseUrl {
    param(
        [string]$Candidate
    )

    if (-not [string]::IsNullOrWhiteSpace($Candidate)) {
        return $Candidate.TrimEnd("/")
    }

    if (-not [string]::IsNullOrWhiteSpace($env:APP_BASE_URL)) {
        return $env:APP_BASE_URL.TrimEnd("/")
    }

    return "http://localhost:8080"
}

function Resolve-TelegramSecret {
    param(
        [string]$Candidate
    )

    if (-not [string]::IsNullOrWhiteSpace($Candidate)) {
        return $Candidate
    }

    if (-not [string]::IsNullOrWhiteSpace($env:TELEGRAM_WEBHOOK_SECRET)) {
        return $env:TELEGRAM_WEBHOOK_SECRET
    }

    throw "Telegram webhook secret is required. Pass -TelegramSecret or set TELEGRAM_WEBHOOK_SECRET in the environment or .env."
}

function New-BaseFromUser {
    return @{
        id            = $TelegramUserId
        is_bot        = $false
        first_name    = $FirstName
        username      = $Username
        language_code = $LanguageCode
    }
}

function New-BaseMessage {
    param(
        [long]$MessageId
    )

    return @{
        message_id = $MessageId
        date       = [int][DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
        chat       = @{
            id   = $ChatId
            type = "private"
        }
    }
}

function New-TelegramUpdateBody {
    $messageId = Get-Random -Minimum 1000 -Maximum 999999
    $updateId = [int](Get-Random -Minimum 100000 -Maximum 999999999)

    switch ($Action) {
        "Start" {
            return @{
                update_id = $updateId
                message   = @{
                    message_id = $messageId
                    date       = [int][DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
                    chat       = @{
                        id   = $ChatId
                        type = "private"
                    }
                    from       = (New-BaseFromUser)
                    text       = "/start"
                    entities   = @(
                        @{
                            offset = 0
                            length = 6
                            type   = "bot_command"
                        }
                    )
                }
            }
        }
        "ViewDetails" {
            if ($ListingId -le 0) {
                throw "-ListingId is required for ViewDetails."
            }

            return @{
                update_id      = $updateId
                callback_query = @{
                    id      = "cb-$updateId"
                    from    = (New-BaseFromUser)
                    message = (New-BaseMessage -MessageId $messageId)
                    data    = "view_details:$ListingId"
                }
            }
        }
        "Buy" {
            if ($ListingId -le 0) {
                throw "-ListingId is required for Buy."
            }

            return @{
                update_id      = $updateId
                callback_query = @{
                    id      = "cb-$updateId"
                    from    = (New-BaseFromUser)
                    message = (New-BaseMessage -MessageId $messageId)
                    data    = "buy_listing:$ListingId"
                }
            }
        }
        "ConfirmBuy" {
            if ($ListingId -le 0) {
                throw "-ListingId is required for ConfirmBuy."
            }

            return @{
                update_id      = $updateId
                callback_query = @{
                    id      = "cb-$updateId"
                    from    = (New-BaseFromUser)
                    message = (New-BaseMessage -MessageId $messageId)
                    data    = "confirm_buy:$ListingId"
                }
            }
        }
        "AnotherDeal" {
            if ($CurrentListingId -le 0) {
                throw "-CurrentListingId is required for AnotherDeal."
            }

            return @{
                update_id      = $updateId
                callback_query = @{
                    id      = "cb-$updateId"
                    from    = (New-BaseFromUser)
                    message = (New-BaseMessage -MessageId $messageId)
                    data    = "another_deal:$CurrentListingId"
                }
            }
        }
        "ContactSupport" {
            return @{
                update_id      = $updateId
                callback_query = @{
                    id      = "cb-$updateId"
                    from    = (New-BaseFromUser)
                    message = (New-BaseMessage -MessageId $messageId)
                    data    = "contact_support:0"
                }
            }
        }
        default {
            throw "Unsupported Telegram action: $Action"
        }
    }
}

function Invoke-TelegramWebhook {
    param(
        [string]$ResolvedBaseUrl,
        [string]$ResolvedTelegramSecret
    )

    $payload = New-TelegramUpdateBody
    $json = $payload | ConvertTo-Json -Depth 10

    if ($ShowPayload) {
        Write-Host "Telegram webhook payload:"
        Write-Host $json
        Write-Host ""
    }

    $response = Invoke-WebRequest `
        -Method Post `
        -Uri "$ResolvedBaseUrl/webhooks/telegram" `
        -Headers @{
            "X-Telegram-Bot-Api-Secret-Token" = $ResolvedTelegramSecret
            "Content-Type"                    = "application/json"
        } `
        -Body $json

    Write-Host "Accepted Telegram webhook simulation."
    Write-Host "Action: $Action"
    Write-Host "HTTP status: $($response.StatusCode)"
    Write-Host "Base URL: $ResolvedBaseUrl"
    Write-Host "Chat ID: $ChatId"
    Write-Host "Telegram user ID: $TelegramUserId"
    if ($ListingId -gt 0) {
        Write-Host "Listing ID: $ListingId"
    }
    if ($CurrentListingId -gt 0) {
        Write-Host "Current listing ID: $CurrentListingId"
    }
    Write-Host ""
    Write-Host "Note: the Telegram webhook handler is asynchronous. Verify the effect in logs, Telegram, or admin."
}

function Invoke-PayPalPage {
    param(
        [string]$ResolvedBaseUrl
    )

    $path = switch ($Action) {
        "PayPalReturn" { "/payments/paypal/return?order_number=$([uri]::EscapeDataString($OrderNumber))" }
        "PayPalCancel" { "/payments/paypal/cancel?order_number=$([uri]::EscapeDataString($OrderNumber))" }
        default { throw "Unsupported PayPal page action: $Action" }
    }

    $response = Invoke-WebRequest -Method Get -Uri "$ResolvedBaseUrl$path"

    Write-Host "Fetched PayPal page simulation."
    Write-Host "Action: $Action"
    Write-Host "HTTP status: $($response.StatusCode)"
    Write-Host "URL: $ResolvedBaseUrl$path"
    Write-Host ""
    Write-Host "Response preview:"
    $content = [string]$response.Content
    if ($content.Length -gt 400) {
        $content = $content.Substring(0, 400)
    }
    Write-Host $content
}

$repoRoot = Get-RepoRoot
Import-DotEnv -Path (Join-Path $repoRoot ".env")

$resolvedBaseUrl = Resolve-BaseUrl -Candidate $BaseUrl

switch ($Action) {
    "Start" {}
    "ViewDetails" {}
    "Buy" {}
    "ConfirmBuy" {}
    "AnotherDeal" {}
    "ContactSupport" {
        $resolvedTelegramSecret = Resolve-TelegramSecret -Candidate $TelegramSecret
        Invoke-TelegramWebhook -ResolvedBaseUrl $resolvedBaseUrl -ResolvedTelegramSecret $resolvedTelegramSecret
        return
    }
    "PayPalReturn" {
        Invoke-PayPalPage -ResolvedBaseUrl $resolvedBaseUrl
        return
    }
    "PayPalCancel" {
        Invoke-PayPalPage -ResolvedBaseUrl $resolvedBaseUrl
        return
    }
    default {
        $resolvedTelegramSecret = Resolve-TelegramSecret -Candidate $TelegramSecret
        Invoke-TelegramWebhook -ResolvedBaseUrl $resolvedBaseUrl -ResolvedTelegramSecret $resolvedTelegramSecret
        return
    }
}

$resolvedTelegramSecret = Resolve-TelegramSecret -Candidate $TelegramSecret
Invoke-TelegramWebhook -ResolvedBaseUrl $resolvedBaseUrl -ResolvedTelegramSecret $resolvedTelegramSecret
