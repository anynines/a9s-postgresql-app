FROM golang:1.15

WORKDIR /app
COPY . /app/
RUN go get github.com/lib/pq

EXPOSE 3000
CMD ["go","run","main.go"]
