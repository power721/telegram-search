FROM node:22-alpine AS web
WORKDIR /src
COPY package.json ./
COPY web/package.json web/package-lock.json ./web/
RUN npm ci --prefix web
COPY web ./web
RUN npm run web:build

FROM golang:1.25-alpine AS go-build
ARG VERSION=dev
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./internal/web/dist
RUN CGO_ENABLED=0 go build -ldflags "-X 'tg-search/internal/build.Version=${VERSION}'" -o /out/tg-search ./cmd/tg-search

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=go-build /out/tg-search /usr/local/bin/tg-search
COPY docker/config.yaml /app/config.yaml
EXPOSE 9900
VOLUME ["/data/tg-search"]
ENTRYPOINT ["tg-search"]
