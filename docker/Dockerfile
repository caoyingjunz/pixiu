FROM golang:1.25-alpine as builder
WORKDIR /app
ARG VERSION
ENV GOPROXY=https://goproxy.cn
COPY ../go.mod ./
COPY ../go.sum ./
#RUN go mod download
COPY .. .
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X 'main.version=${VERSION}'" -o pixiu ./cmd

FROM node:23.7.0-alpine as dashboard-builder
WORKDIR /build
RUN apk add git
RUN git clone https://github.com/caoyingjunz/pixiu-ui
RUN cd pixiu-ui && npm install && npm run build

FROM nginx:1.30.1-alpine as runner
LABEL MAINTAINER="PIXIU"

COPY --from=builder /app/pixiu /app
COPY --from=dashboard-builder /build/pixiu-ui/dist /usr/share/nginx/html
COPY docker/start.sh /usr/local/bin/pixiu_start
COPY docker/nginx-default-80.conf /nginx-default-80.conf

CMD ["pixiu_start"]
