FROM golang:alpine AS build-env

# Download dependencies
WORKDIR /go/src
COPY go.mod go.sum ./
RUN go mod download

# Build current sources
COPY . .
RUN go build -o /go/bin/iperf3_exporter

# Prepare runtime environment
FROM alpine:latest
LABEL maintainer="Edgard Castro <edgardcastro@gmail.com>"
EXPOSE 9579
RUN apk add --no-cache iperf3
COPY --from=build-env /go/bin/iperf3_exporter /bin/
ENTRYPOINT ["/bin/iperf3_exporter"]
