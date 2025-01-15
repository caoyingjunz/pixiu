FROM golang:1.17-alpine as builder
WORKDIR /app
ARG VERSION
ENV GOPROXY=https://goproxy.cn
COPY ../go.mod ./
COPY ../go.sum ./
#RUN go mod download
COPY .. .
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X 'main.version=${VERSION}'" -o pixiu ./cmd

FROM node:16.18.0-alpine as dashboard-builder
WORKDIR /build
RUN apk add git
RUN git clone https://github.com/pixiu-io/dashboard.git
RUN cd dashboard && npm install && npm run build

FROM busybox as runner
LABEL MAINTAINER="PIXIU"

COPY --from=builder /app/pixiu /app
COPY --from=dashboard-builder /build/dashboard/dist /static
COPY docker/start.sh /usr/local/bin/pixiu_start

CMD ["pixiu_start"]
