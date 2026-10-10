#Requires -Version 5.1
<#
.SYNOPSIS
Installs a checksum-verified TryNAS binary on Windows without Go or Node.js.
.EXAMPLE
.\get.ps1 -Source .\dist -AddToPath
.EXAMPLE
.\get.ps1 -Version v1.0.0 -InstallDir "$HOME\bin"
#>
[CmdletBinding()]
param(
    [ValidatePattern('^[A-Za-z0-9._-]+$')]
    [string]$Version = 'latest',
    [string]$InstallDir = '',
    [string]$Source = '',
    [switch]$AddToPath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
if ($env:OS -ne 'Windows_NT') { throw 'Use get.sh on Linux or macOS.' }
if ($Source -and $Version -ne 'latest') { throw '-Source and -Version cannot be combined.' }
if (-not $InstallDir) {
    $local = [Environment]::GetFolderPath('LocalApplicationData')
    $InstallDir = Join-Path $local 'TryNAS\bin'
}
$InstallDir = [IO.Path]::GetFullPath($InstallDir)

# Detect the OS architecture, including ARM64 Windows running an emulated shell.
$nativeArchitecture = $env:PROCESSOR_ARCHITEW6432
if (-not $nativeArchitecture) { $nativeArchitecture = $env:PROCESSOR_ARCHITECTURE }
try {
    $runtimeArchitecture = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
    # Prefer the native Windows environment when older .NET reports an emulated CPU.
    if ($nativeArchitecture -ne 'ARM64' -and $nativeArchitecture -ne 'AMD64') {
        $nativeArchitecture = $runtimeArchitecture
    }
} catch { } # Windows PowerShell 5.1 may not have RuntimeInformation.
switch ($nativeArchitecture.ToUpperInvariant()) {
    'AMD64' { $architecture = 'amd64' }
    'X64' { $architecture = 'amd64' }
    'ARM64' { $architecture = 'arm64' }
    default { throw "Unsupported CPU: $nativeArchitecture. Use AMD64 or ARM64 Windows." }
}
$asset = "trynas-windows-$architecture.exe"
$temporary = Join-Path ([IO.Path]::GetTempPath()) ('trynas-download-' + [Guid]::NewGuid().ToString('N'))
$staged = $null
$previousTls = [Net.ServicePointManager]::SecurityProtocol
try {
    New-Item -ItemType Directory -Path $temporary | Out-Null
    Write-Host "Installing TryNAS for windows/$architecture..."
    if ($Source) {
        Copy-Item -LiteralPath (Join-Path $Source 'SHA256SUMS') -Destination $temporary
        Copy-Item -LiteralPath (Join-Path $Source $asset) -Destination $temporary
    } else {
        [Net.ServicePointManager]::SecurityProtocol = $previousTls -bor [Net.SecurityProtocolType]::Tls12
        $base = 'https://github.com/rajibdpi/trynas/releases/latest/download'
        if ($Version -ne 'latest') { $base = "https://github.com/rajibdpi/trynas/releases/download/$Version" }
        foreach ($name in @('SHA256SUMS', $asset)) {
            Invoke-WebRequest -UseBasicParsing -Uri "$base/$name" -OutFile (Join-Path $temporary $name) -TimeoutSec 300
        }
    }
    $entries = @()
    foreach ($line in (Get-Content -LiteralPath (Join-Path $temporary 'SHA256SUMS'))) {
        $fields = $line.Trim() -split '\s+'
        if ($fields.Count -eq 2 -and ($fields[1] -ceq $asset -or $fields[1] -ceq "*$asset")) {
            $entries += $fields[0]
        }
    }
    if ($entries.Count -ne 1 -or $entries[0] -notmatch '^[0-9A-Fa-f]{64}$') {
        throw "Missing, invalid, or duplicate SHA256SUMS entry for $asset."
    }
    $download = Join-Path $temporary $asset
    $actual = (Get-FileHash -LiteralPath $download -Algorithm SHA256).Hash
    if ($actual -ine $entries[0]) { throw 'Checksum mismatch; existing installation was not replaced.' }
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    $destination = Join-Path $InstallDir 'trynas.exe'
    if (Test-Path -LiteralPath $destination -PathType Container) { throw 'Destination is a directory.' }
    $staged = Join-Path $InstallDir ('.trynas-install-' + [Guid]::NewGuid().ToString('N'))
    Copy-Item -LiteralPath $download -Destination $staged
    # Stage on the destination volume; do not delete the old binary first.
    if ([IO.File]::Exists($destination)) {
        # PowerShell converts $null to an empty string for this .NET parameter.
        # NullString passes a true null backup path, preserving atomic replacement.
        [IO.File]::Replace($staged, $destination, [System.Management.Automation.Language.NullString]::Value)
    } else {
        [IO.File]::Move($staged, $destination)
    }
    $staged = $null
    Write-Host "Installed $destination (SHA-256 verified)."
    if ($AddToPath) {
        $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
        $paths = @($userPath -split ';' | Where-Object { $_ })
        if ($paths -notcontains $InstallDir) {
            [Environment]::SetEnvironmentVariable('Path', (($paths + $InstallDir) -join ';'), 'User')
        }
        if (@($env:Path -split ';') -notcontains $InstallDir) { $env:Path = "$InstallDir;$env:Path" }
        Write-Host 'Added the installation directory to user PATH. Open a new terminal for other sessions.'
    } else {
        Write-Host 'Use -AddToPath to update user PATH, or run the executable by its full path.'
    }
    Write-Host 'Set ADMIN_PASSWORD before the first launch. Existing accounts and data are unchanged.'
} finally {
    [Net.ServicePointManager]::SecurityProtocol = $previousTls
    if ($staged -and (Test-Path -LiteralPath $staged)) { Remove-Item -LiteralPath $staged -Force }
    if (Test-Path -LiteralPath $temporary) { Remove-Item -LiteralPath $temporary -Recurse -Force }
}
