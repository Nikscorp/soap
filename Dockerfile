FROM ghcr.io/nikscorp/go-builder:0.2.0 AS build-backend

ENV \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

ADD . /go/src/soap/

WORKDIR /go/src/soap
RUN VERSION=$(git rev-parse --abbrev-ref HEAD)-$(git log -1 --format=%h) && \
    echo version=$VERSION && \
    go build -ldflags "-X github.com/Nikscorp/soap/internal/pkg/trace.Version=$VERSION" -o soap ./cmd/lazysoap

RUN golangci-lint run ./...
RUN go test -count=1 -v ./...


FROM node:22-alpine AS frontend-deps

ADD frontend/package-lock.json /srv/frontend/package-lock.json
ADD frontend/package.json /srv/frontend/package.json
WORKDIR /srv/frontend
RUN npm ci --no-audit --no-fund


FROM node:22-alpine AS build-frontend

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
