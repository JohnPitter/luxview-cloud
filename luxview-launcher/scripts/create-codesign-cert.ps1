# Gera um certificado Authenticode (autoassinado) para assinar o launcher.
# Uso:
#   pwsh scripts/create-codesign-cert.ps1
# Depois:
#   1) Importe o .pfx no repositório como secret WINDOWS_CERT_BASE64 (base64 do arquivo)
#   2) Secret WINDOWS_CERT_PASSWORD com a senha usada abaixo
#
# Isso troca "Publisher: Unknown" por "LuxView Cloud".
# O SmartScreen do Windows ainda pode avisar até o certificado/reputação serem
# reconhecidos — para sumir de vez, use um certificado OV/EV de uma CA (DigiCert, Sectigo, etc.).

param(
  [string]$OutDir = "$PSScriptRoot\..\certs",
  [string]$Password = "luxview-codesign",
  [string]$Subject = "CN=LuxView Cloud, O=LuxView Cloud, C=BR"
)

$ErrorActionPreference = "Stop"
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$pfx = Join-Path $OutDir "luxview-codesign.pfx"
$cer = Join-Path $OutDir "luxview-codesign.cer"

$secure = ConvertTo-SecureString -String $Password -Force -AsPlainText
$cert = New-SelfSignedCertificate `
  -Type CodeSigningCert `
  -Subject $Subject `
  -CertStoreLocation "Cert:\CurrentUser\My" `
  -KeyExportPolicy Exportable `
  -KeySpec Signature `
  -HashAlgorithm SHA256 `
  -NotAfter (Get-Date).AddYears(3)

Export-PfxCertificate -Cert $cert -FilePath $pfx -Password $secure | Out-Null
Export-Certificate -Cert $cert -FilePath $cer | Out-Null

$b64 = [Convert]::ToBase64String([IO.File]::ReadAllBytes($pfx))
$b64Path = Join-Path $OutDir "luxview-codesign.pfx.b64"
Set-Content -Path $b64Path -Value $b64 -NoNewline

Write-Host "PFX:  $pfx"
Write-Host "CER:  $cer"
Write-Host "B64:  $b64Path"
Write-Host "Pass: $Password"
Write-Host ""
Write-Host "gh secret set WINDOWS_CERT_BASE64 < $b64Path"
Write-Host "gh secret set WINDOWS_CERT_PASSWORD --body `"$Password`""
