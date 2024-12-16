FROM node:16.18.0-alpine as dashboard-builder
WORKDIR /build
RUN apk add --no-cache git
RUN git clone https://github.com/pixiu-io/dashboard.git
RUN cd dashboard && npm install && npm run build

FROM golang:1.17-alpine as builder
WORKDIR /app
ARG VERSION
ENV GOPROXY=https://goproxy.cn
#COPY ./go.mod ./
#COPY ./go.sum ./
#RUN go mod download
COPY . .
COPY --from=dashboard-builder /build/dashboard/dist /app/api/server/router/static
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X 'main.version=${VERSION}'" -o pixiu ./cmd

FROM busybox as runner
COPY --from=builder /app/pixiu /app
ENTRYPOINT ["/app"]
