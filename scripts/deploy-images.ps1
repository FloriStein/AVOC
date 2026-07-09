# deploy-images.ps1 - Baut alle AVOC-Images lokal, verpackt sie, laedt sie in den
# vom CDK-Stack erzeugten S3-Bucket hoch und spielt sie per AWS SSM Run Command
# (kein SSH noetig) auf der EC2-Instanz per 'docker load' ein.
# Windows-native Pendant zu 'make deploy-images' (kein WSL2 noetig).
#
# Voraussetzung: AWS CLI konfiguriert, IAM-Rechte fuer S3 + SSM SendCommand.
#
# Verwendung:
#   .\scripts\deploy-images.ps1 -InstanceId i-0abc123 -Bucket streamingstack-appbucket-xyz
#   .\scripts\deploy-images.ps1 -InstanceId i-0abc123 -Bucket streamingstack-appbucket-xyz -Version 1.1.0
#
# -RemoteUser: Standard ist "ec2-admin" (neuer CDK-Stack, siehe UserData). Bei einer
# bereits laenger laufenden Instanz, die VOR der ec2-admin-Ergaenzung deployed wurde,
# existiert dieser User nicht (UserData laeuft nur beim ersten Boot) - dann:
#   .\scripts\deploy-images.ps1 -InstanceId i-0abc123 -Bucket streamingstack-appbucket-xyz -RemoteUser ec2-user
#
param(
    [Parameter(Mandatory = $true)][string]$InstanceId,
    [Parameter(Mandatory = $true)][string]$Bucket,
    [string]$Region     = "eu-central-1",
    [string]$Version    = "latest",
    [string]$RemoteUser = "ec2-admin"
)

$ErrorActionPreference = "Stop"

$Platform    = "linux/amd64"
$GoServices  = @("control-server", "auth-service", "safety-service", "telemetry-service", "webrtc-sfu")
$TarFile     = "avoc-images.tar"
$TarGzFile   = "avoc-images.tar.gz"
$S3Key       = "bootstrap/avoc-images.tar.gz"

# Von scripts/ aus ins Repo-Root wechseln (Dockerfiles referenzieren Pfade relativ zum Root)
$RepoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $RepoRoot
try {
    Write-Host "[build] Platform: $Platform  Version: $Version"

    foreach ($svc in $GoServices) {
        Write-Host "  -> avoc-$svc"
        docker buildx build --platform $Platform `
            --build-arg SERVICE_NAME=$svc `
            -t "avoc-${svc}:${Version}" `
            -f infrastructure/docker/go-service.Dockerfile . --load
        if ($LASTEXITCODE -ne 0) { throw "Build fehlgeschlagen: avoc-$svc" }
    }

    Write-Host "  -> avoc-vehicle-mock"
    docker buildx build --platform $Platform `
        -t "avoc-vehicle-mock:${Version}" `
        -f infrastructure/docker/vehicle-mock.Dockerfile . --load
    if ($LASTEXITCODE -ne 0) { throw "Build fehlgeschlagen: avoc-vehicle-mock" }

    Write-Host "  -> avoc-frontend"
    docker buildx build --platform $Platform `
        -t "avoc-frontend:${Version}" `
        -f infrastructure/docker/frontend.Dockerfile . --load
    if ($LASTEXITCODE -ne 0) { throw "Build fehlgeschlagen: avoc-frontend" }

    Write-Host "[build] Fertig - 7 Images gebaut fuer $Platform."

    # --- Images in ein Tar-Archiv packen --------------------------------------
    Write-Host ""
    Write-Host "[save] Packe Images in $TarFile ..."

    $allImages = @()
    foreach ($svc in $GoServices) { $allImages += "avoc-${svc}:${Version}" }
    $allImages += "avoc-vehicle-mock:${Version}"
    $allImages += "avoc-frontend:${Version}"

    docker save -o $TarFile @allImages
    if ($LASTEXITCODE -ne 0) { throw "docker save fehlgeschlagen" }

    # --- gzip via .NET (kein externes gzip.exe noetig) ------------------------
    Write-Host "[save] Komprimiere zu $TarGzFile ..."
    $inStream  = [System.IO.File]::OpenRead((Join-Path $RepoRoot $TarFile))
    $outStream = [System.IO.File]::Create((Join-Path $RepoRoot $TarGzFile))
    $gzipStream = New-Object System.IO.Compression.GzipStream($outStream, [System.IO.Compression.CompressionMode]::Compress)
    try {
        $inStream.CopyTo($gzipStream)
    } finally {
        $gzipStream.Dispose()
        $outStream.Dispose()
        $inStream.Dispose()
    }
    Remove-Item $TarFile

    $sizeMb = [math]::Round((Get-Item $TarGzFile).Length / 1MB, 1)
    Write-Host "[save] Fertig: $TarGzFile (${sizeMb} MB)"

    # --- Nach S3 hochladen -----------------------------------------------------
    Write-Host ""
    Write-Host "[upload] Lade nach s3://$Bucket/$S3Key hoch ..."
    aws s3 cp $TarGzFile "s3://$Bucket/$S3Key" --region $Region
    if ($LASTEXITCODE -ne 0) { throw "S3-Upload fehlgeschlagen" }

    # --- Per SSM Run Command auf der Instanz laden (kein SSH/scp noetig) ------
    # Parameter als JSON-Datei uebergeben statt als Inline-String - vermeidet
    # fehleranfaelliges manuelles Escaping verschachtelter Anfuehrungszeichen.
    Write-Host ""
    Write-Host "[load] Starte Remote-Kommando auf $InstanceId als Benutzer '$RemoteUser' (SSM Run Command) ..."
    $innerScript   = "cd /home/$RemoteUser/app && aws s3 cp s3://$Bucket/$S3Key . --region $Region && gunzip -c avoc-images.tar.gz | docker load && rm avoc-images.tar.gz"
    $remoteCommand = "sudo -u $RemoteUser bash -c '$innerScript'"

    # Set-Content -Encoding utf8 wuerde in Windows PowerShell 5.1 IMMER ein UTF-8-BOM
    # schreiben - die AWS CLI stolpert darueber beim Parsen von --parameters file://.
    # Deshalb explizit ueber .NET ohne BOM schreiben (funktioniert in 5.1 und 7+ gleich).
    $paramsFile = Join-Path $env:TEMP "avoc-ssm-params.json"
    $paramsJson = @{ commands = @($remoteCommand) } | ConvertTo-Json -Depth 5
    [System.IO.File]::WriteAllText($paramsFile, $paramsJson, (New-Object System.Text.UTF8Encoding $false))

    $commandId = aws ssm send-command `
        --instance-ids $InstanceId `
        --document-name "AWS-RunShellScript" `
        --parameters "file://$paramsFile" `
        --region $Region `
        --query "Command.CommandId" --output text
    Remove-Item $paramsFile -ErrorAction SilentlyContinue
    if ($LASTEXITCODE -ne 0) { throw "ssm send-command fehlgeschlagen" }

    Write-Host "[load] Command-ID: $commandId - warte auf Abschluss..."
    aws ssm wait command-executed --command-id $commandId --instance-id $InstanceId --region $Region
    $waitExit = $LASTEXITCODE

    $status = aws ssm get-command-invocation --command-id $commandId --instance-id $InstanceId `
        --region $Region --query "Status" --output text
    $stdout = aws ssm get-command-invocation --command-id $commandId --instance-id $InstanceId `
        --region $Region --query "StandardOutputContent" --output text
    $stderr = aws ssm get-command-invocation --command-id $commandId --instance-id $InstanceId `
        --region $Region --query "StandardErrorContent" --output text

    Write-Host "--- Remote-Ausgabe ---"
    Write-Host $stdout
    if ($stderr) { Write-Host "--- Remote-Fehlerausgabe ---"; Write-Host $stderr }

    if ($status -ne "Success") {
        if ($stderr -match "unknown user $RemoteUser") {
            throw "SSM-Kommando fehlgeschlagen: Benutzer '$RemoteUser' existiert auf dieser Instanz nicht (UserData laeuft nur beim ersten Boot). Bei einer aelteren, bereits laufenden Instanz: -RemoteUser ec2-user verwenden."
        }
        throw "SSM-Kommando auf der Instanz fehlgeschlagen (Status: $status)"
    }

    Write-Host ""
    Write-Host "[load] Fertig. Deploy auf EC2: aws ssm start-session --target $InstanceId --region $Region"
}
finally {
    Pop-Location
}
