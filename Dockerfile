ARG alpine_version=3.24
ARG golang_version=1.26.3
FROM golang:${golang_version} AS build
ARG revision=dev
WORKDIR /src
COPY go.mod go.sum ./
COPY main.go ./
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-X main.Version=${revision}" -o /pubcrawl .

FROM alpine:${alpine_version}
RUN apk add --no-cache ca-certificates
COPY --from=build /pubcrawl /usr/local/bin/pubcrawl
USER 65534:65534
ENTRYPOINT ["/usr/local/bin/pubcrawl"]
