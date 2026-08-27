FROM golang:1.25-alpine AS builder
WORKDIR /app
ARG VERSION
ENV GOPROXY=https://goproxy.cn
COPY .. .
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X 'main.version=${VERSION}'" -o pixiu ./cmd

FROM node:22.13.0-alpine AS dashboard-builder
WORKDIR /build
COPY pixiu-ui .
RUN npm install --ignore-scripts && npx vite build --mode docker

FROM nginx:1.30.1-alpine AS runner
LABEL MAINTAINER="PIXIU"

COPY --from=builder /app/pixiu /app
COPY --from=dashboard-builder /build/dist /usr/share/nginx/html
COPY docker/start.sh /usr/local/bin/pixiu_start

CMD ["pixiu_start"]
