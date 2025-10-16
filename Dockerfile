FROM golang:1.25.1-alpine3.22 AS builder

ARG SHA1="[no-sha]"
ARG TAG="[no-tag]"
ARG TARGETOS
ARG TARGETARCH
ARG TARGETPLATFORM

WORKDIR /go/src/peerdb_exporter

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN BUILD_DATE=$(date +%F-%T) CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /peerdb_exporter \
    -ldflags  "-s -w -extldflags \"-static\" -X main.BuildVersion=$TAG -X main.BuildCommitSha=$SHA1 -X main.BuildDate=$BUILD_DATE" .

FROM alpine:3.21 AS runner

COPY --from=builder /peerdb_exporter /peerdb_exporter

USER 3001:3001

EXPOSE 8001
ENTRYPOINT ["/peerdb_exporter"]
