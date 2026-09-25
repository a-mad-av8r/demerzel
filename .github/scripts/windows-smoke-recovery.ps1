function Enter-WindowsSmoke {
  param([string]$InstallDir, [string]$ConfigDir)

  # Only one smoke test can use the fixed service name; the system releases the mutex when its process exits.
  $mutex = [System.Threading.Mutex]::new($false, "Global\GPTLoad-Windows-Smoke")
  $acquired = $false
  try {
    try { $acquired = $mutex.WaitOne(0) }
    catch [System.Threading.AbandonedMutexException] { $acquired = $true }
    if (-not $acquired) { throw "another Windows smoke is running" }

    $owners = @()
    foreach ($directory in @($InstallDir, $ConfigDir)) {
      if (-not (Test-Path -LiteralPath $directory)) { continue }
      if ((Get-Item -LiteralPath $directory -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) {
        throw "refusing linked Windows smoke directory: $directory"
      }
      foreach ($name in @('.service-smoke-owner', '.installer-smoke-owner')) {
        $path = Join-Path $directory $name
        if (-not (Test-Path -LiteralPath $path)) { continue }
        $item = Get-Item -LiteralPath $path -Force
        if ($item.PSIsContainer -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
          throw "refusing invalid Windows smoke marker: $path"
        }
        $token = (Get-Content -LiteralPath $path -Raw).Trim()
        if ($token -notmatch '^[0-9a-fA-F]{32}$') { throw "invalid Windows smoke owner: $path" }
        $owners += @{ Name = $name; Token = $token }
      }
    }
    # Without a marker, leave the state intact so the caller's existing preflight rejects a real installation.
    if ($owners.Count -eq 0) { return $mutex }
    $owner = $owners[0]
    foreach ($other in $owners) {
      if ($other.Name -ne $owner.Name -or $other.Token -ne $owner.Token) {
        throw "conflicting Windows smoke owners"
      }
    }
    $installer = $owner.Name -eq '.installer-smoke-owner'
    if ($installer) {
      # The installer removes and recreates its installation directory; its ownership proof is stored in ProgramData, which survives uninstallation.
      $proof = Join-Path $ConfigDir 'data/installer-smoke-failure.txt'
      if (-not (Test-Path -LiteralPath $proof) -or
          ((Get-Content -LiteralPath $proof -Raw).Trim() -ne $owner.Token)) {
        throw "missing installer smoke ownership proof"
      }
    } else {
      foreach ($directory in @($InstallDir, $ConfigDir)) {
        if ((Test-Path -LiteralPath $directory) -and
            -not (Test-Path -LiteralPath (Join-Path $directory $owner.Name))) {
          throw "unmarked Windows smoke directory: $directory"
        }
      }
    }

    $binary = Join-Path $InstallDir 'gpt-load.exe'
    $service = Get-CimInstance Win32_Service -Filter "Name='gpt-load'" -ErrorAction Stop
    if ($null -ne $service) {
      if ($service.PathName -ine ('"' + $binary + '" service run') -or
          $service.StartName -ine 'NT AUTHORITY\LocalService') {
        throw "refusing Windows service with unexpected configuration"
      }
      $controller = Get-Service -Name 'gpt-load' -ErrorAction Stop
      try {
        if ($controller.Status -ne 'Stopped') {
          $controller.Stop()
          $controller.WaitForStatus('Stopped', [TimeSpan]::FromSeconds(15))
        }
      } finally { $controller.Dispose() }
      & sc.exe delete 'gpt-load' | Out-Null
      if ($LASTEXITCODE -ne 0) { throw "failed to remove stale Windows smoke service" }
    }

    # Reclaim only fixed targets with confirmed ownership; never scan or clean up other tasks, services, or directories.
    if (Test-Path -LiteralPath $InstallDir) { Remove-Item -LiteralPath $InstallDir -Recurse -Force }
    if ($installer) {
      $paths = @(
        ([IO.Path]::Combine([Environment]::GetFolderPath('CommonDesktopDirectory'), 'GPT-Load.url')),
        ([IO.Path]::Combine([Environment]::GetFolderPath('CommonPrograms'), 'GPT-Load/GPT-Load.url')),
        'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\{E5A127DE-2676-4F6C-B763-CF53C6271883}_is1'
      )
      foreach ($path in $paths) {
        if (Test-Path -LiteralPath $path) { Remove-Item -LiteralPath $path -Recurse -Force }
      }
    }
    # Remove the ownership-marker directory last; on intermediate failure, retain the evidence needed for the next recovery.
    if (Test-Path -LiteralPath $ConfigDir) { Remove-Item -LiteralPath $ConfigDir -Recurse -Force }
    return $mutex
  } catch {
    if ($acquired) { $mutex.ReleaseMutex() }
    $mutex.Dispose()
    throw
  }
}
