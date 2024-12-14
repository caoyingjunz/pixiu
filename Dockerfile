FROM golang:1.17-alpine as builder
WORKDIR /app
ARG VERSION
ENV GOPROXY=https://goproxy.cn
COPY ./go.mod ./
COPY ./go.sum ./
#RUN go mod download
COPY . .
RUN set -eux; \
	apt-get update; \
	apt-get install -y --no-install-recommends \
		g++ \
		gcc \
		libc6-dev \
		make \
		pkg-config \
	; \
	rm -rf /var/lib/apt/lists/*
RUN CGO_ENABLED=1 go build -ldflags "-s -w -X 'main.version=${VERSION}'" -o pixiu ./cmd

FROM busybox as runner
COPY --from=builder /app/pixiu /app
ENTRYPOINT ["/app"]
