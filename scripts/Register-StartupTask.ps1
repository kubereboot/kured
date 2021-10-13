Write-Host "Verifying container is a host process container"
# TODO 

Write-Host "Creating startup task"

Get-ScheduledTask -TaskName 'kured-at-startup-task' -ErrorAction SilentlyContinue | Unregister-ScheduledTask -Confirm:$False

$action = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument '-NoProfile -WindowStyle Hidden -Command "& { if (Test-Path /var/run/reboot-required) { Remove-Item -Force -Path /var/run/reboot-required }}"' 
$principal = New-ScheduledTaskPrincipal -UserId SYSTEM -LogonType ServiceAccount
$trigger = New-JobTrigger -AtStartup -RandomDelay 00:00:05
$definition = New-ScheduledTask -Action $action -Principal $principal -Trigger $trigger -Description "kured-at-startup-task"
Register-ScheduledTask -TaskName "kured-at-startup-task" -InputObject $definition