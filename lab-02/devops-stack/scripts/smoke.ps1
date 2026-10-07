$ErrorActionPreference = 'Stop'
$stackRoot = Split-Path -Parent $PSScriptRoot
Push-Location $stackRoot
try {
    $port = 8443
    foreach ($line in Get-Content -LiteralPath '.env') {
        if ($line -match '^HTTPS_PORT=(\d+)$') { $port = [int]$Matches[1] }
    }
    $base = "https://localhost:$port"
    $healthText = & curl.exe -fsSk "$base/health"
    if ($LASTEXITCODE -ne 0) { throw 'Health request failed' }
    $health = $healthText | ConvertFrom-Json
    if ($health.status -ne 'ok' -or -not $health.db) { throw 'Database is not ready' }
    $payloadPath = Join-Path ([IO.Path]::GetTempPath()) ('lab02-note-' + [guid]::NewGuid() + '.json')
    try {
        [IO.File]::WriteAllText($payloadPath, '{"title":"Smoke check","body":"HTTPS -> nginx -> Go -> PostgreSQL"}', (New-Object Text.UTF8Encoding $false))
        $createdText = & curl.exe -fsSk -X POST "$base/api/notes" -H 'Content-Type: application/json' --data-binary "@$payloadPath"
        if ($LASTEXITCODE -ne 0) { throw 'Create request failed' }
        $created = $createdText | ConvertFrom-Json
        $noteText = & curl.exe -fsSk "$base/api/notes/$($created.id)"
        if ($LASTEXITCODE -ne 0) { throw 'Get request failed' }
        $note = $noteText | ConvertFrom-Json
        if ($note.title -ne 'Smoke check') { throw 'Stored note differs' }
        $listText = & curl.exe -fsSk "$base/api/notes"
        if ($LASTEXITCODE -ne 0) { throw 'List request failed' }
        $notes = @($listText | ConvertFrom-Json)
        if (-not ($notes | Where-Object { $_.id -eq $created.id })) { throw 'Created note missing from list' }
        Write-Host "PASS: health, POST, GET by id, list; note id=$($created.id)"
    } finally { Remove-Item -LiteralPath $payloadPath -ErrorAction SilentlyContinue }
} finally { Pop-Location }
