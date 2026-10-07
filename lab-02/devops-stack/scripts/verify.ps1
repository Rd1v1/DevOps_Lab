$ErrorActionPreference = 'Stop'
$stackRoot = Split-Path -Parent $PSScriptRoot
$evidenceRoot = Join-Path (Split-Path -Parent $stackRoot) 'evidence'
New-Item -ItemType Directory -Force -Path $evidenceRoot | Out-Null
Push-Location $stackRoot
function Run-Docker {
    param([string[]]$DockerArgs)
    & docker @DockerArgs
    if ($LASTEXITCODE -ne 0) { throw "Docker command failed: $($DockerArgs[0])" }
}
function Record {
    param([string]$Name, [scriptblock]$Action)
    Write-Host "Checking $Name"
    & $Action 2>&1 | Tee-Object -FilePath (Join-Path $evidenceRoot $Name)
}
try {
    Record '04-api-runtime.log' {
        Run-Docker @('compose','-p','devops-lab02','config','--quiet')
        & "$PSScriptRoot/smoke.ps1"
        Run-Docker @('compose','-p','devops-lab02','ps','-a')
        Run-Docker @('compose','-p','devops-lab02','top','backend')
        Run-Docker @('exec','devops-lab02-backend-1','sh','-c','id; printf "PID1: "; tr "\000" " " < /proc/1/cmdline; echo')
        Run-Docker @('inspect','devops-lab02-backend-1','--format','memory={{.HostConfig.Memory}} nano_cpus={{.HostConfig.NanoCpus}} user={{.Config.User}}')
        Run-Docker @('compose','-p','devops-lab02','stats','--no-stream')
        & curl.exe -fsSk https://localhost:8443/health
        & curl.exe -fsSk https://localhost:8443/api/notes
        & curl.exe -sSI http://localhost:8088/
        Run-Docker @('run','--rm','--network','devops-lab02_default','devops-lab02-cert-generator','sh','-c','printf "" | openssl s_client -connect nginx:8443 -servername localhost -showcerts 2>&1')
    }
    Record '05-persistence-and-shutdown.log' {
        $before = & curl.exe -fsSk https://localhost:8443/api/notes
        if ($LASTEXITCODE -ne 0) { throw 'Cannot read notes before restart' }
        Run-Docker @('compose','-p','devops-lab02','stop','backend')
        Run-Docker @('compose','-p','devops-lab02','logs','--tail','10','backend')
        $state = & docker inspect devops-lab02-backend-1 --format '{{json .State}}' | ConvertFrom-Json
        "Shutdown ExitCode=$($state.ExitCode) OOMKilled=$($state.OOMKilled)"
        if ($state.ExitCode -ne 0 -or $state.OOMKilled) { throw 'Backend did not shut down cleanly' }
        Run-Docker @('compose','-p','devops-lab02','down')
        Run-Docker @('compose','-p','devops-lab02','up','-d','--wait','--wait-timeout','60')
        $after = & curl.exe -fsSk https://localhost:8443/api/notes
        if ($LASTEXITCODE -ne 0 -or ($before -join '') -ne ($after -join '')) { throw 'Notes did not survive container recreation' }
        'PASS: same notes after down/up; named volume preserved'
    }
    Record '06-localhost-failure.log' {
        $argsFault=@('compose','-p','lab02-localhost','-f','docker-compose.yml','-f','diagnostics/images.yml','-f','diagnostics/localhost.yml')
        Run-Docker ($argsFault+@('up','-d','--no-build','db','backend'))
        try {
            for ($i=0; $i -lt 25; $i++) {
                $health=& docker inspect lab02-localhost-backend-1 --format '{{.State.Health.Status}}'
                if ($health -eq 'unhealthy') { break }
                Start-Sleep -Seconds 2
            }
            Run-Docker ($argsFault+@('ps','-a'))
            Run-Docker ($argsFault+@('logs','--tail','15','backend'))
            Run-Docker @('inspect','lab02-localhost-backend-1','--format','{{json .State}}')
            Run-Docker @('top','lab02-localhost-backend-1')
            # Nonzero wget status is expected here (HTTP 503).
            & docker exec lab02-localhost-backend-1 wget -S -O - http://127.0.0.1:8000/health
            if ($LASTEXITCODE -eq 0 -or $health -ne 'unhealthy') { throw 'Wrong DSN did not fail readiness' }
            Run-Docker @('exec','lab02-localhost-backend-1','sh','-c','getent hosts db; nc -z -w 2 db 5432; if nc -z -w 2 localhost 5432; then exit 1; fi')
            Run-Docker @('compose','-p','lab02-localhost','-f','docker-compose.yml','-f','diagnostics/images.yml','up','-d','--no-build','--wait','--wait-timeout','60','db','backend')
            Run-Docker @('exec','lab02-localhost-backend-1','wget','-qO-','http://127.0.0.1:8000/health')
            'PASS: localhost failed; removing overlay restored db DNS and readiness'
        } finally { Run-Docker ($argsFault+@('down')) }
    }
    Record '07-startup-race.log' {
        $argsFault=@('compose','-p','lab02-race','-f','docker-compose.yml','-f','diagnostics/images.yml','-f','diagnostics/race.yml')
        Run-Docker ($argsFault+@('up','-d','--no-build','db','backend'))
        try {
            Start-Sleep -Seconds 4
            Run-Docker ($argsFault+@('ps','-a'))
            Run-Docker ($argsFault+@('logs','--timestamps','--tail','15'))
            & docker exec lab02-race-backend-1 wget -S -O - http://127.0.0.1:8000/health
            if ($LASTEXITCODE -eq 0) { throw 'Race was not observed' }
            Run-Docker @('inspect','lab02-race-backend-1','--format','{{json .State}}')
            Run-Docker @('top','lab02-race-backend-1')
            for ($i=0; $i -lt 25; $i++) {
                $health=& docker inspect lab02-race-backend-1 --format '{{.State.Health.Status}}'
                if ($health -eq 'healthy') { break }
                Start-Sleep -Seconds 2
            }
            if ($health -ne 'healthy') { throw 'Database did not recover after startup delay' }
            Run-Docker @('exec','lab02-race-backend-1','wget','-qO-','http://127.0.0.1:8000/health')
            Run-Docker ($argsFault+@('logs','--timestamps','--tail','15'))
            'PASS: backend started before DB; health was 503 then recovered to 200'
            Run-Docker @('compose','-p','lab02-race','-f','docker-compose.yml','-f','diagnostics/images.yml','up','-d','--no-build','--wait','--wait-timeout','60','db','backend')
        } finally { Run-Docker ($argsFault+@('down')) }
    }
    Record '08-oom.log' {
        $argsFault=@('compose','-p','lab02-oom','-f','docker-compose.yml','-f','diagnostics/oom.yml')
        $since=[DateTimeOffset]::UtcNow.ToUnixTimeSeconds().ToString()
        Run-Docker ($argsFault+@('up','-d','--build','db','backend'))
        try {
            for ($i=0; $i -lt 30; $i++) {
                $status=& docker inspect lab02-oom-backend-1 --format '{{.State.Status}}'
                if ($status -eq 'exited') { break }
                Start-Sleep -Seconds 1
            }
            Run-Docker ($argsFault+@('ps','-a'))
            Run-Docker ($argsFault+@('logs','--tail','20','backend'))
            Run-Docker @('inspect','lab02-oom-backend-1','--format','{{json .State}}')
            Run-Docker @('events','--since',$since,'--until',[DateTimeOffset]::UtcNow.ToUnixTimeSeconds().ToString(),'--filter','label=com.docker.compose.project=lab02-oom')
            $state=& docker inspect lab02-oom-backend-1 --format '{{json .State}}' | ConvertFrom-Json
            if ($state.ExitCode -ne 137 -or -not $state.OOMKilled) { throw 'OOM was not proven' }
            'PASS: 64 MiB cgroup limit; OOMKilled=true and exit 137'
        } finally { Run-Docker ($argsFault+@('down')) }
    }
    Record '09-disk-usage.log' { Run-Docker @('system','df'); Run-Docker @('system','df','-v') }
} finally { Pop-Location }
