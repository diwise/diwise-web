FROM --platform=$BUILDPLATFORM docker.io/golang:1.26 AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

RUN go install github.com/a-h/templ/cmd/templ@v0.3.1001

ARG BUILDARCH

RUN echo $BUILDARCH

RUN case "$BUILDARCH" in \
    amd64) TAILWIND_ARCH=x64 ;; \
    arm64) TAILWIND_ARCH=arm64 ;; \
    *) TAILWIND_ARCH="$BUILDARCH" ;; \
  esac \
  && curl --fail -sL "https://github.com/tailwindlabs/tailwindcss/releases/download/v4.2.2/tailwindcss-linux-$TAILWIND_ARCH" -o tailwindcss \
  && chmod +x tailwindcss

COPY go.mod .
COPY go.sum .

COPY . .

RUN go mod download

RUN templ generate
RUN ./tailwindcss -i ./internal/presentation/web/css/input.css -o ./assets/css/diwise.css

WORKDIR /app/cmd/diwise-web

ARG TARGETARCH
ARG TARGETOS

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build


FROM registry.access.redhat.com/ubi9/ubi-minimal
WORKDIR /opt/diwise

COPY --from=builder --chown=1001 /app/cmd/diwise-web/diwise-web /opt/diwise
COPY --from=builder --chown=1001 /app/assets /opt/diwise/assets

RUN chown 1001 /opt/diwise

EXPOSE 8080
USER 1001

ENTRYPOINT ["/opt/diwise/diwise-web"]
