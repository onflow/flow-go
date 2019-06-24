FROM node:9-alpine

WORKDIR /home/node/app

COPY ./package* ./

RUN npm install && \
    npm cache clean --force && \
    npm install -g mocha

COPY . .

CMD ["mocha", "tests/*.js", "--reporter", "spec"]
