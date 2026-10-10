# Native Windows tests: powershell -NoProfile -File scripts/install/powershell_test.ps1
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$installer = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..\get.ps1'))
$tokens = $null
$parseErrors = $null
[System.Management.Automation.Language.Parser]::ParseFile($installer, [ref]$tokens, [ref]$parseErrors) | Out-Null
if ($parseErrors.Count -ne 0) { throw "PowerShell syntax errors: $parseErrors" }
if ($env:OS -ne 'Windows_NT') { Write-Host 'Syntax passed; native installation tests require Windows.'; exit 0 }
$temporary = Join-Path ([IO.Path]::GetTempPath()) ('trynas-ps-test-' + [Guid]::NewGuid().ToString('N'))
$oldArchitecture = $env:PROCESSOR_ARCHITECTURE
$oldNative = $env:PROCESSOR_ARCHITEW6432
$oldPath = $env:Path
try {
    New-Item -ItemType Directory -Path $temporary | Out-Null
    foreach ($architecture in @('AMD64', 'ARM64')) {
        $env:PROCESSOR_ARCHITECTURE = $architecture
        $env:PROCESSOR_ARCHITEW6432 = $architecture
        $source = Join-Path $temporary "source-$architecture"
        $destination = Join-Path $temporary "install with spaces-$architecture"
        New-Item -ItemType Directory -Path $source | Out-Null
        $asset = "trynas-windows-$($architecture.ToLowerInvariant()).exe"
        $binary = Join-Path $source $asset
        [IO.File]::WriteAllText($binary, 'test release')
        $hash = (Get-FileHash -LiteralPath $binary -Algorithm SHA256).Hash
        $manifest = Join-Path $source 'SHA256SUMS'
        [IO.File]::WriteAllText($manifest, "$hash  $asset`n")
        & $installer -Source $source -InstallDir $destination
        $installed = Join-Path $destination 'trynas.exe'
        if ([IO.File]::ReadAllText($installed) -ne 'test release') { throw 'Incorrect installed payload.' }
        # A second valid install exercises replacement of an existing executable.
        [IO.File]::WriteAllText($binary, 'updated test release')
        $hash = (Get-FileHash -LiteralPath $binary -Algorithm SHA256).Hash
        [IO.File]::WriteAllText($manifest, "$hash  $asset`n")
        & $installer -Source $source -InstallDir $destination
        if ([IO.File]::ReadAllText($installed) -ne 'updated test release') { throw 'Existing binary was not upgraded.' }
        [IO.File]::WriteAllText($binary, 'tampered')
        $rejected = $false
        try { & $installer -Source $source -InstallDir $destination } catch { $rejected = $true }
        if (-not $rejected) { throw 'Tampered binary accepted.' }
        if ([IO.File]::ReadAllText($installed) -ne 'updated test release') { throw 'Existing binary changed after failed verification.' }
        [IO.File]::WriteAllText($manifest, "$hash  $asset`n$hash  $asset`n")
        $rejected = $false
        try { & $installer -Source $source -InstallDir $destination } catch { $rejected = $true }
        if (-not $rejected) { throw 'Duplicate checksum accepted.' }
        if ($env:Path -cne $oldPath) { throw 'PATH changed without -AddToPath.' }
    }
    Write-Host 'PowerShell installation tests passed.'
} finally {
    $env:PROCESSOR_ARCHITECTURE = $oldArchitecture
    $env:PROCESSOR_ARCHITEW6432 = $oldNative
    $env:Path = $oldPath
    Remove-Item -LiteralPath $temporary -Recurse -Force
}
