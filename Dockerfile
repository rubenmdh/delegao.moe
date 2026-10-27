FROM golang:1.27-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOTOOLCHAIN=local
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /delegao .
RUN mkdir -p /state/public && chown -R 65532:65532 /state

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /delegao /delegao
COPY --from=build --chown=65532:65532 /state /state
ENV PUBLIC_DIR=/state/public BUILD_WORKERS=2 DB_MAX_OPEN_CONNS=4 DB_MAX_IDLE_CONNS=1
ENV GOMEMLIMIT=64MiB GOMAXPROCS=2
USER 65532:65532
VOLUME ["/state"]
EXPOSE 8080
ENTRYPOINT ["/delegao"]
