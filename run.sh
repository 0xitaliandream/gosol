docker run -d \
    --name clickhouse \
    -p 8123:8123 \
    -p 9000:9000 \
    --ulimit nofile=262144:262144 \
    clickhouse/clickhouse-server


curl http://localhost:8123/ping