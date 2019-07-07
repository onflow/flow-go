FROM node:9-alpine

WORKDIR /home/node/app

COPY ./integration-tests/package* ./
COPY ./proto ./proto

RUN npm install && \
    npm cache clean --force && \
    npm install -g mocha

COPY ./integration-tests .

CMD ["mocha", "tests/*.js", "--reporter", "spec"]
