# TODO: figure out if we can use Windows machines to build the images
# or docker buildkit to build on windows machines.
ARG BASE_OS_VERSION
FROM --platform=linux/amd64 alpine:3.14 as prep

FROM mcr.microsoft.com/windows/nanoserver:${BASE_OS_VERSION}
COPY ./Kured-Init.ps1 /Kured-Init.ps1
COPY ./Test-PendingReboot.ps1 /Test-PendingReboot.ps1
COPY ./kured.exe /kured.exe
ENV PATH="C:\Windows\system32;C:\Windows;C:\WINDOWS\System32\WindowsPowerShell\v1.0\;"
ENTRYPOINT ["/kured.exe"]
