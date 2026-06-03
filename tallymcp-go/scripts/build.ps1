# TallyMCP Go Build Script
# Produces tallymcp.exe - a single binary MCP server for TallyPrime

$ErrorActionPreference = "Stop"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  TallyMCP Go - Build Script" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

# Refresh PATH to pick up Go installation
$env:Path = [System.Environment]::GetEnvironmentVariable("Path","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path","User")

# Verify Go is available
Write-Host "`n[1/3] Checking Go installation..." -ForegroundColor Yellow
go version
if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: Go is not installed or not in PATH!" -ForegroundColor Red
    exit 1
}

# Run go mod tidy
Write-Host "`n[2/3] Resolving dependencies..." -ForegroundColor Yellow
go mod tidy
if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: go mod tidy failed!" -ForegroundColor Red
    exit 1
}

# Build the executable
Write-Host "`n[3/3] Building tallymcp.exe..." -ForegroundColor Yellow
go build -o tallymcp.exe .
if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: Build failed!" -ForegroundColor Red
    exit 1
}

$exe = Get-Item "tallymcp.exe"
$sizeMB = [math]::Round($exe.Length / 1MB, 1)

Write-Host "`n========================================" -ForegroundColor Green
Write-Host "  BUILD SUCCESS!" -ForegroundColor Green
Write-Host "  Output: tallymcp.exe ($sizeMB MB)" -ForegroundColor Green
Write-Host "  No dependencies required to run!" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host "`nRegister in Claude Desktop config:" -ForegroundColor White
Write-Host '  "tally-mcp-server": {' -ForegroundColor Gray
Write-Host "    `"command`": `"$($exe.FullName -replace '\\','\\')`"" -ForegroundColor Gray
Write-Host '  }' -ForegroundColor Gray
