FROM nikscorp/go-builder:0.0.1 as build-backend

ENV \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

ADD app /go/src/soap/app/
ADD vendor /go/src/soap/vendor/
ADD .golangci.yml /go/src/soap

WORKDIR /go/src/soap
RUN go build -o soap soap/app
RUN golangci-lint run ./...


FROM node:13.10.1-alpine3.11 as frontend-deps

ADD frontend/package-lock.json /srv/frontend/package-lock.json
ADD frontend/package.json /srv/frontend/package.json
WORKDIR /srv/frontend
RUN npm ci


FROM node:13.10.1-alpine3.11 as build-frontend

ADD frontend /srv/frontend
COPY --from=frontend-deps /srv/frontend/node_modules /srv/frontend/node_modules
WORKDIR /srv/frontend
RUN npm run build


FROM alpine:3.11
LABEL maintainer="Nikscorp <voynov@nikscorp.com>"

COPY --from=build-backend /go/src/soap/soap /srv/soap
COPY --from=build-frontend /srv/frontend/build /static/

CMD ["/srv/soap"]
EXPOSE 8080