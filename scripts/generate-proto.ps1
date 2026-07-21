$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
protoc -I "$root/contracts/proto" --go_out="$root/contracts" --go_opt=module=github.com/clivegformer/platform/contracts --go-grpc_out="$root/contracts" --go-grpc_opt=module=github.com/clivegformer/platform/contracts "$root/contracts/proto/user/v1/user.proto" "$root/contracts/proto/file/v1/file.proto" "$root/contracts/proto/storage/v1/storage.proto" "$root/contracts/proto/analysis/v1/analysis.proto"
& "$root/analysis_srv/.venv/Scripts/python.exe" -m grpc_tools.protoc -I "$root/contracts/proto" --python_out="$root/analysis_srv/generated" --grpc_python_out="$root/analysis_srv/generated" "$root/contracts/proto/file/v1/file.proto" "$root/contracts/proto/storage/v1/storage.proto" "$root/contracts/proto/analysis/v1/analysis.proto"
if ($LASTEXITCODE -ne 0) { throw 'Python protobuf generation failed' }
