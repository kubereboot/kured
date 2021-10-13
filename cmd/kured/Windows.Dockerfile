# TODO: figure out if we can use Windows machines to build the images
# or docker buildkit to build on windows machines.
#FROM --platform=linux/amd64 alpine:3.14 as prep

FROM mcr.microsoft.com/windows/nanoserver:1809-amd64
COPY ./Register-StartupTask.ps1 /Register-StartupTask.ps1
COPY ./Test-PendingReboot.ps1 /Test-PendingReboot.ps1
COPY ./kured.exe /kured.exe
#ENV PATH=c:\\Windows\\System32;c:\\Windows # Only needed for x-arch builds
ENTRYPOINT ["/kured.exe"]
