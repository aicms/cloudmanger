# Build the Go backend and place it as a Tauri sidecar (filename must carry the target triple).
# Run before `tauri build` (or just use `pnpm build`).
$ErrorActionPreference = "Stop"

$root = Resolve-Path (Join-Path $PSScriptRoot "..")
$triple = (rustc --print host-tuple).Trim()

# Copy the frontend build output into the backend embed dir (Go uses go:embed),
# otherwise the sidecar will not contain the latest frontend.
$webuiDist = Join-Path $root "backend/internal/webui/dist"
$frontendDist = Join-Path $root "frontend/dist"
if (-not (Test-Path $frontendDist)) {
    Write-Error "frontend/dist not found. Run 'pnpm build' in the frontend directory first."
}
Write-Host "Copying frontend/dist -> $webuiDist"
if (Test-Path $webuiDist) { Remove-Item -Recurse -Force $webuiDist }
New-Item -ItemType Directory -Force -Path $webuiDist | Out-Null
Copy-Item -Recurse "$frontendDist/*" $webuiDist

# Map target triple -> Go cross-compile args.
$goos = "windows"
$goarch = "amd64"
if ($triple -like "aarch64*") { $goarch = "arm64" }
elseif ($triple -like "i686*") { $goarch = "386" }
elseif ($triple -like "x86_64*" -or $triple -like "i586*") { $goarch = "amd64" }
else { Write-Error "Cannot infer GOARCH from triple: $triple" }

$outDir = Join-Path $PSScriptRoot "src-tauri/binaries"
New-Item -ItemType Directory -Force -Path $outDir | Out-Null
$out = Join-Path $outDir "cloud-manager-$triple.exe"

$env:GOOS = $goos
$env:GOARCH = $goarch
$env:CGO_ENABLED = "0"

Write-Host "Building Go sidecar -> $out  (GOOS=$goos GOARCH=$goarch triple=$triple)"
Push-Location (Join-Path $root "backend")
try {
    go build -trimpath -ldflags="-s -w" -o $out ./cmd/server
} finally {
    Pop-Location
}
Write-Host "Done."
