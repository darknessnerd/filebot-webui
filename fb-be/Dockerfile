FROM node:22.8.0-alpine3.20
#INATALL NGINX AND SUPERVISOR
RUN apk update
RUN apk add nginx
RUN apk add supervisor
#FILE BOT CLI SETUP
ENV CONFIG="/config"
ENV FILEBOT_OPTS="-Dapplication.deployment=docker -Duser.home=$CONFIG"
ENV FILEBOT_VERSION=5.1.5
ENV FILEBOT_URL=https://get.filebot.net/filebot/FileBot_${FILEBOT_VERSION}/FileBot_${FILEBOT_VERSION}-portable.tar.xz
# Add an environment variable for the license file path
ENV FILEBOT_LICENSE_PATH="/config/filebot/license.psm"


RUN apk --no-cache add openjdk11 --repository=http://dl-cdn.alpinelinux.org/alpine/edge/community
RUN \
        mkdir /opt/filebot && \
        echo "**** Fetch filebot package ****" && \
        wget -O /tmp/filebot.tar.xz "$FILEBOT_URL" && \
        echo "**** Extract application files ****" && \
        tar --extract --file /tmp/filebot.tar.xz --directory /opt/filebot --verbose && \
        echo "**** Cleanup ****" && \
        find /opt/filebot/lib -type f -not -name libjnidispatch.so -delete && \
        rm -rf \
                $HOME/.cache \
                /tmp/* && \
        echo "**** Create filebot config and binary symlinks ****" && \
        ln -s $CONFIG /opt/filebot/data
# Ensure the symlink is created
RUN ln -sf /opt/filebot/filebot.sh /usr/bin/filebot && \
    chmod +x /usr/bin/filebot
#REPPLACE NGINX DEFAULT WITH YOUR CODE
RUN rm -f /etc/nginx/http.d/default.conf
ADD ./docker/nginx/http.d/default.conf /etc/nginx/http.d/default.conf

#COPY YOUR SUPERVISOR CONFIG FILES INSIDE SUPERVISOR FOLDER
COPY ./docker/supervisord.conf /etc/supervisor/supervisord.conf
COPY ./docker/supervisor.conf /etc/supervisor/conf.d/supervisor.conf

#MAKE WORKING DIRECTORY AND LOGS DIRECTORY
RUN mkdir -p /home/www/node/node_modules && chown -R node:node /home/www/node
RUN mkdir -p /var/log/supervisor && chown -R node:node /var/log/supervisor

#INSTALL AND RUN NPM
WORKDIR /home/www/node
COPY package*.json ./
RUN npm install
RUN npm ci --only=production
COPY --chown=node:node . ./

# Create entrypoint script to handle FileBot license setup
COPY ./docker/start.sh  /start.sh
RUN chmod +x /start.sh

EXPOSE "3001"

# CMD - Start supervisord with entrypoint
ENTRYPOINT ["/start.sh"]
CMD ["/usr/bin/supervisord", "-n", "-c", "/etc/supervisor/supervisord.conf"]
