FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w -extldflags '-static'" -o /app/webui-be ./cmd/webui-be


FROM alpine:3.19

RUN apk add --no-cache \
    ca-certificates \
    openjdk11-jre-headless \
    wget \
    tar \
    xz \
    mediainfo \
    libmediainfo \
    libzen

ENV CONFIG="/config"
ENV FILEBOT_OPTS="-Dapplication.deployment=docker -Duser.home=$CONFIG"
ENV FILEBOT_VERSION=5.1.7
ENV FILEBOT_URL=https://get.filebot.net/filebot/FileBot_${FILEBOT_VERSION}/FileBot_${FILEBOT_VERSION}-portable.tar.xz
ENV FILEBOT_LICENSE_PATH="/config/filebot/license.psm"

RUN mkdir -p /opt/filebot && \
    wget -O /tmp/filebot.tar.xz "$FILEBOT_URL" && \
    tar --extract --file /tmp/filebot.tar.xz --directory /opt/filebot && \
    if [ -d /opt/filebot/lib ]; then \
      find /opt/filebot/lib -type f -not -name libjnidispatch.so -delete; \
    fi && \
    rm -rf /tmp/* && \
    ln -s $CONFIG /opt/filebot/data && \
    ln -sf /opt/filebot/filebot.sh /usr/bin/filebot && \
    chmod +x /usr/bin/filebot

WORKDIR /app
COPY --from=builder /app/webui-be /app/webui-be
COPY docker/start.sh /start.sh
RUN chmod +x /start.sh && mkdir -p /data

EXPOSE 8080

ENTRYPOINT ["/start.sh"]
CMD ["/app/webui-be"]
