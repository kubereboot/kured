# TODO: figure out if we can use Windows machines to build the images
# or docker buildkit to build on windows machines.
ARG BASE_OS_VERSION
FROM --platform=linux/amd64 alpine:3.14 as prep

FROM mcr.microsoft.com/windows/nanoserver:${BASE_OS_VERSION}
COPY ./Register-StartupTask.ps1 /Register-StartupTask.ps1
COPY ./Test-PendingReboot.ps1 /Test-PendingReboot.ps1
COPY ./kured.exe /kured.exe
ENV PATH="c:\\Windows\\System32;c:\\Windows;"
ENTRYPOINT ["/kured.exe"]
