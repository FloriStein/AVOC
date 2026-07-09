# setup-ssm.ps1 — Legt alle AVOC SSM Parameter unter /avoc/prod/ an.
# Windows-native Pendant zu setup-ssm.sh (kein WSL2/Bash nötig).
# Einmalig vom Dev-Rechner ausführen. Voraussetzung: aws cli konfiguriert + IAM-Rechte.
#
# Verwendung:
#   $env:AWS_REGION = "eu-central-1"
#   .\scripts\setup-ssm.ps1
#
$ErrorActionPreference = "Stop"

$Region = if ($env:AWS_REGION) { $env:AWS_REGION } else { "eu-central-1" }

function ConvertFrom-SecureStringPlain {
    param([System.Security.SecureString]$SecureString)
    $bstr = [System.Runtime.InteropServices.Marshal]::SecureStringToBSTR($SecureString)
    try {
        return [System.Runtime.InteropServices.Marshal]::PtrToStringAuto($bstr)
    } finally {
        [System.Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
    }
}

function Put-SecureParam {
    param([string]$Name, [string]$Value)
    aws ssm put-parameter --region $Region --name $Name --value $Value `
        --type SecureString --overwrite --query "Version" --output text | Out-Null
    Write-Host "  [SecureString] $Name"
}

function Put-StringParam {
    param([string]$Name, [string]$Value)
    aws ssm put-parameter --region $Region --name $Name --value $Value `
        --type String --overwrite --query "Version" --output text | Out-Null
    Write-Host "  [String]       $Name"
}

Write-Host "=== AVOC SSM Parameter Store Setup ==="
Write-Host "Region: $Region"
Write-Host ""
Write-Host "Bitte Werte eingeben (Enter = Standard uebernehmen wo angegeben):"
Write-Host ""

# JWT Secret
$jwtSecret = Read-Host "JWT_SECRET (min. 32 Zeichen, kein Default)"
if ($jwtSecret.Length -lt 32) {
    Write-Host "ERROR: JWT_SECRET muss mindestens 32 Zeichen lang sein."
    exit 1
}

# Elastic IP
$turnExternalIp = Read-Host "TURN_EXTERNAL_IP (Elastic IP der EC2-Instanz)"
if ([string]::IsNullOrEmpty($turnExternalIp)) {
    Write-Host "ERROR: TURN_EXTERNAL_IP darf nicht leer sein."
    exit 1
}

# TURN
$turnRealm = Read-Host "TURN_REALM [avoc.example.com]"
if ([string]::IsNullOrEmpty($turnRealm)) { $turnRealm = "avoc.example.com" }

$turnUser = Read-Host "TURN_USER [avoc]"
if ([string]::IsNullOrEmpty($turnUser)) { $turnUser = "avoc" }

$turnPasswordSecure = Read-Host "TURN_PASSWORD (min. 16 Zeichen)" -AsSecureString
$turnPassword = ConvertFrom-SecureStringPlain $turnPasswordSecure
if ($turnPassword.Length -lt 16) {
    Write-Host "ERROR: TURN_PASSWORD muss mindestens 16 Zeichen lang sein."
    exit 1
}

# Grafana
$grafanaUser = Read-Host "GRAFANA_ADMIN_USER [admin]"
if ([string]::IsNullOrEmpty($grafanaUser)) { $grafanaUser = "admin" }

$grafanaPasswordSecure = Read-Host "GRAFANA_ADMIN_PASSWORD (min. 12 Zeichen)" -AsSecureString
$grafanaPassword = ConvertFrom-SecureStringPlain $grafanaPasswordSecure
if ($grafanaPassword.Length -lt 12) {
    Write-Host "ERROR: GRAFANA_ADMIN_PASSWORD muss mindestens 12 Zeichen lang sein."
    exit 1
}

# PostgreSQL (ADR-023)
$dbPasswordSecure = Read-Host "DB_PASSWORD (min. 16 Zeichen - PostgreSQL avoc-User)" -AsSecureString
$dbPassword = ConvertFrom-SecureStringPlain $dbPasswordSecure
if ($dbPassword.Length -lt 16) {
    Write-Host "ERROR: DB_PASSWORD muss mindestens 16 Zeichen lang sein."
    exit 1
}

# Admin-User Seed (ADR-024)
$adminPasswordSecure = Read-Host "ADMIN_PASSWORD (min. 12 Zeichen - initialer Admin-Account)" -AsSecureString
$adminPassword = ConvertFrom-SecureStringPlain $adminPasswordSecure
if ($adminPassword.Length -lt 12) {
    Write-Host "ERROR: ADMIN_PASSWORD muss mindestens 12 Zeichen lang sein."
    exit 1
}

# MediaMTX WHIP Stream Key (ADR-020)
$whipStreamKeySecure = Read-Host "WHIP_STREAM_KEY (min. 32 Zeichen - Bearer Token fuer Larix)" -AsSecureString
$whipStreamKey = ConvertFrom-SecureStringPlain $whipStreamKeySecure
if ($whipStreamKey.Length -lt 32) {
    Write-Host "ERROR: WHIP_STREAM_KEY muss mindestens 32 Zeichen lang sein."
    exit 1
}

Write-Host ""
Write-Host "Schreibe Parameter nach /avoc/prod/ ..."
Write-Host ""

Put-SecureParam "/avoc/prod/jwt-secret"             $jwtSecret
Put-SecureParam "/avoc/prod/db-password"            $dbPassword
Put-SecureParam "/avoc/prod/admin-password"         $adminPassword
Put-SecureParam "/avoc/prod/whip-stream-key"        $whipStreamKey
Put-StringParam "/avoc/prod/turn-external-ip"       $turnExternalIp
Put-StringParam "/avoc/prod/turn-realm"             $turnRealm
Put-StringParam "/avoc/prod/turn-user"              $turnUser
Put-SecureParam "/avoc/prod/turn-password"          $turnPassword
Put-StringParam "/avoc/prod/grafana-admin-user"     $grafanaUser
Put-SecureParam "/avoc/prod/grafana-admin-password" $grafanaPassword

Write-Host ""
Write-Host "=== Alle Parameter angelegt. ==="
Write-Host ""
Write-Host "Naechster Schritt: Images bauen und Konfiguration uebertragen (siehe ec2-bootstrap-windows.md):"
Write-Host "  .\scripts\deploy-images.ps1 -EC2Host ec2-admin@<IP> -SSHKey ~\.ssh\avoc-ec2-keypair.pem"
Write-Host "  scp -i ~\.ssh\avoc-ec2-keypair.pem scripts\deploy.sh ec2-admin@<IP>:~/app/"
Write-Host "  ssh -i ~\.ssh\avoc-ec2-keypair.pem ec2-admin@<IP> 'bash ~/app/deploy.sh'"
