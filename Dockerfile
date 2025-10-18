# Stage 1: Build
FROM golang:alpine AS builder
WORKDIR /code
COPY . .
RUN go mod tidy && go build -o app .

# Stage 2: Run
FROM alpine:latest
WORKDIR /
COPY --from=builder /code/app /app
ENTRYPOINT ["/app"]
