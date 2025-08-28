FROM golang:tip-alpine3.22

# Install required packages: nginx, openjdk11, wget, tar, xz
RUN apk update && \
    apk add --no-cache nginx openjdk11 wget tar xz gcc libc-dev

# FileBot CLI setup
ENV CONFIG="/config"
ENV FILEBOT_OPTS="-Dapplication.deployment=docker -Duser.home=$CONFIG"
ENV FILEBOT_VERSION=5.1.7
ENV FILEBOT_URL=https://get.filebot.net/filebot/FileBot_${FILEBOT_VERSION}/FileBot_${FILEBOT_VERSION}-portable.tar.xz
ENV FILEBOT_LICENSE_PATH="/config/filebot/license.psm"
ENV CGO_ENABLED=1

RUN mkdir /opt/filebot && \
    echo "**** Fetch filebot package ****" && \
    wget -O /tmp/filebot.tar.xz "$FILEBOT_URL" && \
    if [ ! -f /tmp/filebot.tar.xz ]; then echo "FileBot archive not downloaded!"; exit 1; fi && \
    echo "**** Extract application files ****" && \
    tar --extract --file /tmp/filebot.tar.xz --directory /opt/filebot --verbose && \
    echo "**** List extracted files for debugging ****" && \
    ls -l /opt/filebot && \
    if [ -d /opt/filebot/lib ]; then \
      find /opt/filebot/lib -type f -not -name libjnidispatch.so -delete; \
    fi && \
    rm -rf $HOME/.cache /tmp/* && \
    echo "**** Create filebot config and binary symlinks ****" && \
    ln -s $CONFIG /opt/filebot/data

RUN ln -sf /opt/filebot/filebot.sh /usr/bin/filebot && \
    chmod +x /usr/bin/filebot

# Build Go application
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY .env .
COPY cmd /app/cmd
COPY internal ./internal
RUN go build -o main /app/cmd/webui-be
RUN chmod +x /app/main

# Copy start script
COPY docker/start.sh /start.sh

# Server Configuration
ENV SERVER_HOST=0.0.0.0
ENV SERVER_PORT=8080

# Database Configuration
ENV DB_TYPE=sqlite
ENV DB_DATABASE=/app/app.db
RUN chmod +x /start.sh

EXPOSE 8080

# Entrypoint: run start.sh (setup FileBot license, then run Go backend)
ENTRYPOINT ["/start.sh"]
CMD ["/app/main"]
