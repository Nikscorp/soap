FROM nikscorp/go-builder:0.1.7 as build-backend

ENV \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

ADD . /go/src/soap/

WORKDIR /go/src/soap
RUN VERSION=$(git rev-parse --abbrev-ref HEAD)-$(git log -1 --format=%h) && \
    echo version=$VERSION && \
    go build -ldflags "-X github.com/Nikscorp/soap/internal/pkg/trace.Version=$VERSION" -o soap ./cmd/lazysoap && \
    sed -i "s/OVERRIDE_VERSION/$VERSION/g" swagger/swagger.yaml

RUN golangci-lint run ./...
RUN go test -count=1 -v ./...


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
COPY --from=build-backend /go/src/soap/swagger /swagger

CMD ["/srv/soap"]
EXPOSE 8080
