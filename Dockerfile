FROM golang:1.16 as builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build main.go

FROM ubuntu:hirsute
WORKDIR /app/
COPY --from=builder /app/main .

RUN apt-get update && apt-get install -y git

ENTRYPOINT [ "./main" ]
