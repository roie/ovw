$ErrorActionPreference = "Stop"

$Repo = "roie/ovw"
$Bin = "ovw"
$InstallDir = if ($env:OVW_INSTALL_DIR) { $env:OVW_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "ovw\bin" }
$RequestedVersion = if ($env:OVW_VERSION) { $env:OVW_VERSION } else { "latest" }

$Arch = switch ($env:PROCESSOR_ARCHITECTURE) {
  "AMD64" { "amd64" }
  "ARM64" { "arm64" }
  default {
    throw "unsupported architecture: $env:PROCESSOR_ARCHITECTURE"
  }
}

if ($RequestedVersion -eq "latest") {
  $Release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
  $Tag = $Release.tag_name
} else {
  $Tag = $RequestedVersion
}

if (-not $Tag) {
  throw "could not resolve latest ovw release"
}

$Version = $Tag.TrimStart("v")
$Asset = "${Bin}_${Version}_windows_${Arch}.zip"
$Url = "https://github.com/$Repo/releases/download/$Tag/$Asset"
$Temp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid())

New-Item -ItemType Directory -Force -Path $Temp | Out-Null

try {
  $Zip = Join-Path $Temp $Asset
  Write-Host "Downloading $Asset"
  Invoke-WebRequest -Uri $Url -OutFile $Zip
  Expand-Archive -Path $Zip -DestinationPath $Temp -Force

  New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
  $Source = Get-ChildItem -Path $Temp -Filter "$Bin.exe" -Recurse | Select-Object -First 1
  if (-not $Source) {
    throw "$Bin.exe not found in $Asset"
  }

  Copy-Item $Source.FullName (Join-Path $InstallDir "$Bin.exe") -Force
  Write-Host "Installed $Bin to $InstallDir\$Bin.exe"

  $PathParts = ($env:PATH -split ";") | Where-Object { $_ }
  if ($PathParts -notcontains $InstallDir) {
    Write-Host "Add $InstallDir to PATH if needed."
  }
} finally {
  Remove-Item -Recurse -Force $Temp -ErrorAction SilentlyContinue
}
