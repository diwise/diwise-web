FROM --platform=$BUILDPLATFORM docker.io/golang:1.22 AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

RUN go install github.com/a-h/templ/cmd/templ@v0.2.663

ARG BUILDARCH
ARG BUILDOS
ARG TARGETARCH
ARG TARGETOS

RUN curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/download/v3.4.3/tailwindcss-$BUILDOS-$BUILDARCH
RUN mv tailwindcss-$BUILDOS-$BUILDARCH tailwindcss
RUN chmod +x tailwindcss

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN templ generate
RUN ./tailwindcss -c ./internal/pkg/presentation/web/css/tailwind.config.js -i ./internal/pkg/presentation/web/css/input.css -o ./assets/css/diwise.css

WORKDIR /app/cmd/diwise-web

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build


FROM registry.access.redhat.com/ubi9/ubi-minimal
WORKDIR /opt/diwise

COPY --from=builder --chown=1001 /app/cmd/diwise-web/diwise-web /opt/diwise
COPY --from=builder --chown=1001 /app/assets /opt/diwise/assets

RUN chown 1001 /opt/diwise

EXPOSE 8080
USER 1001

ENTRYPOINT ["/opt/diwise/diwise-web"]
