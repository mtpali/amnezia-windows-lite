param(
    [ValidateSet('amd64','arm64','386')][string]$Arch = 'amd64',
    [switch]$RunTests
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
Set-Location $PSScriptRoot
$tools = Join-Path $PSScriptRoot '.tools'
New-Item -ItemType Directory -Path $tools -Force | Out-Null
$goRoot = Join-Path $tools 'go'
$go = Join-Path $goRoot 'bin/go.exe'
if (!(Test-Path $go)) {
    $archive = Join-Path $tools 'go.zip'
    Invoke-WebRequest 'https://go.dev/dl/go1.25.14.windows-amd64.zip' -OutFile $archive
    if ((Get-FileHash $archive -Algorithm SHA256).Hash.ToLowerInvariant() -ne '119044a92b3987c341cd6aebb256676dd4780d292f7b4e72a3e9976677841697') { throw 'Go archive checksum mismatch' }
    tar -xf $archive -C $tools
    if ($LASTEXITCODE -ne 0) { throw 'Go extraction failed' }
    Remove-Item $archive
}
$env:GOROOT = $goRoot
$env:GOPATH = Join-Path $tools 'gopath'
$env:GOTOOLCHAIN = 'local'
$env:CGO_ENABLED = '0'
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
if ($RunTests) {
    & $go run github.com/akavel/rsrc@v0.10.2 -arch amd64 -manifest manifest.xml -ico 'ui/icon/lite.ico,ui/icon/dot.ico' -o resources_windows_amd64.syso
    if ($LASTEXITCODE -ne 0) { throw 'Test resource generation failed' }
    $env:LITE_UI_SMOKE = '1'
    $env:LITE_SCREENSHOT_DIR = Join-Path $PSScriptRoot 'dist/ui'
    & $go test ./internal/...
    if ($LASTEXITCODE -ne 0) { throw 'Preference tests failed' }
    & $go test . -run '^TestLite' -count=1
    if ($LASTEXITCODE -ne 0) { throw 'Configuration policy tests failed' }
    # UI package tests are a separate executable and need their own v6 common-controls manifest.
    Copy-Item resources_windows_amd64.syso ui/resources_windows_amd64.syso
    try {
        & $go test ./ui/... -run '^TestLite' -count=1 -timeout 45s
        if ($LASTEXITCODE -ne 0) { throw 'Editor tests failed' }
    } finally {
        Remove-Item ui/resources_windows_amd64.syso -ErrorAction SilentlyContinue
    }
}
& $go run github.com/akavel/rsrc@v0.10.2 -arch $Arch -manifest manifest.xml -ico 'ui/icon/lite.ico,ui/icon/dot.ico' -o "resources_windows_$Arch.syso"
if ($LASTEXITCODE -ne 0) { throw 'Resource generation failed' }
$env:GOARCH = $Arch
$output = Join-Path $PSScriptRoot "dist/windows-lite-$Arch"
New-Item -ItemType Directory -Path $output -Force | Out-Null
& $go build -ldflags='-H windowsgui -s -w' -trimpath -buildvcs=false -o "$output/WindowsLite.exe" .
if ($LASTEXITCODE -ne 0) { throw 'Build failed' }
$wintunZip = Join-Path $tools 'wintun.zip'
if (!(Test-Path $wintunZip)) { Invoke-WebRequest 'https://www.wintun.net/builds/wintun-0.14.1.zip' -OutFile $wintunZip }
if ((Get-FileHash $wintunZip -Algorithm SHA256).Hash.ToLowerInvariant() -ne '07c256185d6ee3652e09fa55c0b673e2624b565e02c4b9091c79ca7d2f24ef51') { throw 'Wintun archive checksum mismatch' }
Expand-Archive $wintunZip -DestinationPath $tools -Force
$wintunArch = if ($Arch -eq '386') { 'x86' } else { $Arch }
Copy-Item "$tools/wintun/bin/$wintunArch/wintun.dll" $output
Copy-Item COPYING,PORTABLE.txt,THIRD-PARTY-NOTICES.txt $output
$zip = Join-Path $PSScriptRoot "dist/amnezia-windows-lite-$Arch.zip"
$files = @('WindowsLite.exe','wintun.dll','COPYING','PORTABLE.txt','THIRD-PARTY-NOTICES.txt') | ForEach-Object { Join-Path $output $_ }
Compress-Archive -Path $files -DestinationPath $zip -Force
$hash = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLowerInvariant()
"$hash  amnezia-windows-lite-$Arch.zip" | Set-Content "dist/SHA256-$Arch.txt" -Encoding ascii
Write-Host "Built $zip"
