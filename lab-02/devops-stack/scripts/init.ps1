$ErrorActionPreference = 'Stop'
$stackRoot = Split-Path -Parent $PSScriptRoot
Push-Location $stackRoot
try {
    if (-not (Test-Path -LiteralPath '.env')) {
        $randomBytes = New-Object byte[] 32
        $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
        try { $rng.GetBytes($randomBytes) } finally { $rng.Dispose() }
        $password = [Convert]::ToBase64String($randomBytes).TrimEnd('=').Replace('+','-').Replace('/','_')
        $envText = "DB_PASSWORD=$password`nHTTPS_PORT=8443`nHTTP_PORT=8088`n"
        [IO.File]::WriteAllText((Join-Path $stackRoot '.env'), $envText, (New-Object Text.UTF8Encoding $false))
        Write-Host 'Created local .env (password is not printed).'
    }
    if (-not (Test-Path -LiteralPath 'nginx/ssl/server.key') -or -not (Test-Path -LiteralPath 'nginx/ssl/server.crt')) {
        New-Item -ItemType Directory -Force -Path 'nginx/ssl' | Out-Null
        docker build --target openssl-builder -t devops-lab02-cert-generator ./nginx
        if ($LASTEXITCODE -ne 0) { throw 'Certificate helper build failed' }
        $certPath = (Resolve-Path -LiteralPath 'nginx/ssl').Path
        docker run --rm --mount "type=bind,source=$certPath,target=/work/ssl" devops-lab02-cert-generator
        if ($LASTEXITCODE -ne 0) { throw 'Certificate generation failed' }
    }
    docker compose config --quiet
    if ($LASTEXITCODE -ne 0) { throw 'Compose configuration is invalid' }
    Write-Host 'Ready: docker compose -p devops-lab02 up -d --build --wait'
} finally { Pop-Location }
