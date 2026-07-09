ARG SERVICE_NAME

FROM golang:1.23-alpine AS builder
# ARG must be re-declared after FROM to be available in this stage
ARG SERVICE_NAME
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the specific service (gen/ is committed — no protoc step needed)
RUN go build -o /service ./cmd/${SERVICE_NAME}

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /service /service
EXPOSE 8080
ENTRYPOINT ["/service"]
