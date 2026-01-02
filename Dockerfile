FROM alpine:3.21
RUN apk add --no-cache iperf3
COPY iperf3_exporter /iperf3_exporter
USER nobody:nobody
ENTRYPOINT ["/iperf3_exporter"]
