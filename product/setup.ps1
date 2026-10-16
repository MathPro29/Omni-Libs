$ErrorActionPreference = 'Stop'
$productRoot = $PSScriptRoot
$source = Join-Path $productRoot 'vendor/Omni-Libs'

if (-not (Test-Path -LiteralPath $source)) {
    New-Item -ItemType Directory -Force -Path (Split-Path $source) | Out-Null
    git clone --depth 1 https://github.com/MathPro29/Omni-Libs.git $source
    if ($LASTEXITCODE -ne 0) { throw 'Could not clone Omni-Libs from GitHub.' }
}

Push-Location $productRoot
try {
    npm install --no-audit --no-fund
    if ($LASTEXITCODE -ne 0) { throw 'Could not install the OmniLogs package.' }
} finally {
    Pop-Location
}

Write-Host "Omni-Libs commit: $(git -C $source rev-parse --short HEAD)"
