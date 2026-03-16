FROM golang:1.24-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /9level-collector ./cmd/collector

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /9level-collector /usr/local/bin/9level-collector
COPY frontend/ /frontend/
RUN mkdir -p /data
EXPOSE 3001
ENTRYPOINT ["9level-collector"]
