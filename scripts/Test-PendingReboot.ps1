<#
    Return 0 / SUCCESS if reboot is required!
#>

$cbsKey = Get-ItemProperty "HKLM:\Software\Microsoft\Windows\CurrentVersion\Component Based Servicing" -ErrorAction Ignore
if ($cbsKey.PSObject.Properties.name -contains 'RebootPending') {
    Write-Output "Reboot requried - Component Based Servicing:RebootPending registry value detected"
    New-Item -Path '/var/run' -Name 'reboot-required' -ItemType File -Force
}

$autoUpdateKey = Get-ItemProperty "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Auto Update" -ErrorAction Ignore
if ($autoUpdateKey.PSObject.Properties.name -contains 'RebootRequired') {
    Write-Output "Reboot required - Auto Update:RebootRequired registry value detected"
    New-Item -Path '/var/run' -Name 'reboot-required' -ItemType File -Force
}

if (Test-Path -Path '/var/run/reboot-required') {
    Write-Output "Reboot required"
    exit 0
}

Write-Output "Reboot not required"
exit 1
