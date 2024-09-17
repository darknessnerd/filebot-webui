FROM node:22.8.0-alpine3.20


#INATALL NGINX AND SUPERVISOR
RUN apk update
RUN apk add nginx
RUN apk add supervisor

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
EXPOSE "3001"

CMD ["/usr/bin/supervisord", "-n", "-c", "/etc/supervisor/supervisord.conf"]
