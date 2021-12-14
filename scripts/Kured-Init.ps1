$ErrorActionPreference = "Stop"

Write-Host "Verifying container is a host process container"
if ($env:KURED_NODE_ID -ne $env:COMPUTERNAME) {
    throw "Container is not running as a hostprocess container!"
}

Write-Host "Creating startup task"

Get-ScheduledTask -TaskName 'kured-at-startup-task' -ErrorAction SilentlyContinue | Unregister-ScheduledTask -Confirm:$False

$action = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument '-NoProfile -WindowStyle Hidden -Command "& { if (Test-Path /var/run/reboot-required) { Remove-Item -Force -Path /var/run/reboot-required }}"' 
$principal = New-ScheduledTaskPrincipal -UserId SYSTEM -LogonType ServiceAccount
$trigger = New-JobTrigger -AtStartup -RandomDelay 00:00:05
$definition = New-ScheduledTask -Action $action -Principal $principal -Trigger $trigger -Description "kured-at-startup-task"
Register-ScheduledTask -TaskName "kured-at-startup-task" -InputObject $definition

Write-Host "Creating kubeconfig.conf from pod's service account creds"

$server = [Environment]::ExpandEnvironmentVariables("https://%KUBERNETES_SERVICE_HOST%:%KUBERNETES_SERVICE_PORT_HTTPS%")
$kubeconfig = @"
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority: ca.crt
    server: $server
  name: default
contexts:
- context:
    cluster: default
    namespace: default
    user: default
  name: default
current-context: default
users:
- name: default
  user:
    tokenFile: token
"@

$kubeconfig | Out-File -encoding ASCII -filepath "$env:CONTAINER_SANDBOX_MOUNT_POINT/var/run/secrets/kubernetes.io/serviceaccount/kubeconfig.conf"