FROM alpine:3.23
RUN apk add --no-cache iperf3
COPY iperf3_exporter /iperf3_exporter
USER nobody:nobody
ENTRYPOINT ["/iperf3_exporter"]
