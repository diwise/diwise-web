FROM --platform=$BUILDPLATFORM docker.io/golang:1.21 AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

RUN go install github.com/a-h/templ/cmd/templ@v0.2.476

ARG TARGETARCH
ARG TARGETOS

RUN curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-$TARGETOS-$TARGETARCH
RUN chmod +x tailwindcss-$TARGETOS-$TARGETARCH
RUN mv tailwindcss-$TARGETOS-$TARGETARCH tailwindcss

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN templ generate
RUN ./tailwindcss -c ./css/tailwind.config.js -i ./css/input.css -o ./css/output.css

WORKDIR /app/cmd/diwise-web

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build


FROM registry.access.redhat.com/ubi9/ubi-minimal
WORKDIR /opt/diwise

COPY --from=builder --chown=1001 /app/cmd/diwise-web/diwise-web /opt/diwise
COPY --from=builder --chown=1001 /app/css/output.css /opt/diwise/css/output.css
COPY --from=builder --chown=1001 /app/js/htmx.min.js /opt/diwise/js/htmx.min.js

RUN chown 1001 /opt/diwise

EXPOSE 8080
USER 1001

ENTRYPOINT ["/opt/diwise/diwise-web"]
