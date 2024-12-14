FROM golang:1.17 as builder
WORKDIR /app
ARG VERSION
ENV GOPROXY=https://goproxy.cn
COPY ./go.mod ./
COPY ./go.sum ./
#RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -ldflags "-s -w -X 'main.version=${VERSION}'" -o pixiu ./cmd

FROM jacky06/static:nonroot as runner
COPY --from=builder /app/pixiu /app
ENTRYPOINT ["/app"]
