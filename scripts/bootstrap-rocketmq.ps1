$ErrorActionPreference = 'Stop'
docker exec clivegformer-rocketmq-broker sh -c "sh mqadmin updateTopic -n host.docker.internal:9876 -c DefaultCluster -t FILE_UPLOAD_COMPLETE"
Write-Host 'RocketMQ topic FILE_UPLOAD_COMPLETE is ready.'

