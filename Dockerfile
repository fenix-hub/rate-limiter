FROM golang:latest

WORKDIR /app

COPY . .

RUN go mod download

RUN go build -o /rate-limiter

EXPOSE 8080

CMD [ "/rate-limiter" ]