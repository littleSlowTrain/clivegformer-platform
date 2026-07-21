$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
docker run --rm -v "${root}:/src" -v clivegformer-go-mod-cache:/go/pkg/mod -v clivegformer-go-build-cache:/root/.cache/go-build -w /src -e GOOS=windows -e GOARCH=amd64 -e CGO_ENABLED=0 golang:1.25 sh -c "mkdir -p bin && go build -o bin/user_srv.exe ./user_srv && go build -o bin/file_srv.exe ./file_srv && go build -o bin/storage_srv.exe ./storage_srv && go build -o bin/admin.exe ./storage_srv/cmd/doctor && go build -o bin/repair.exe ./storage_srv/cmd/repair && go build -o bin/gin_web.exe ./gin_web"
Write-Host 'Windows service binaries are ready in bin/.'
