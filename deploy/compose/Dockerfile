FROM golang:1.27-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/bookmarker-web ./cmd/bookmarker-web && \
    CGO_ENABLED=0 go build -o /out/bookmarker-hash ./cmd/bookmarker-hash

FROM node:24-bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libasound2 \
    libatk-bridge2.0-0 \
    libatk1.0-0 \
    libcairo-gobject2 \
    libcairo2 \
    libcups2 \
    libdbus-1-3 \
    libdrm2 \
    libfontconfig1 \
    libfreetype6 \
    libgbm1 \
    libgdk-pixbuf-2.0-0 \
    libglib2.0-0 \
    libgtk-3-0 \
    libnspr4 \
    libnss3 \
    libnss3-tools \
    libpango-1.0-0 \
    libpangocairo-1.0-0 \
    libx11-6 \
    libx11-xcb1 \
    libxcb-shm0 \
    libxcb1 \
    libxcomposite1 \
    libxcursor1 \
    libxdamage1 \
    libxext6 \
    libxfixes3 \
    libxi6 \
    libxkbcommon0 \
    libxrandr2 \
    libxrender1 \
    libxshmfence1 \
    fonts-freefont-ttf \
    fonts-noto-cjk \
    fonts-noto-color-emoji && \
    rm -rf /var/lib/apt/lists/* && \
    npm install --global agent-browser@0.35.2 && \
    agent-browser install

WORKDIR /app
COPY --from=build /out/bookmarker-web /usr/local/bin/bookmarker-web
COPY --from=build /out/bookmarker-hash /usr/local/bin/bookmarker-hash
COPY migrations ./migrations
COPY web ./web
COPY deploy/compose/config.yaml.tmpl ./config.yaml.tmpl
COPY deploy/compose/entrypoint.sh /usr/local/bin/bookmarker-entrypoint
RUN chmod 755 /usr/local/bin/bookmarker-entrypoint && \
    mkdir -p /var/lib/bookmarker/screenshots

EXPOSE 8080
ENTRYPOINT ["bookmarker-entrypoint"]
CMD ["bookmarker-web"]