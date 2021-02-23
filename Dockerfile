FROM golang:1.16

WORKDIR /app
COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

COPY main.go main.go
COPY templates templates
COPY public public

EXPOSE 3000
CMD ["go","run","main.go"]
