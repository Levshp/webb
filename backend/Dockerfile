# syntax=docker/dockerfile:1

FROM golang:1.23-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/app main.go

FROM gcr.io/distroless/static-debian12
WORKDIR /app

COPY --from=build /out/app /app/app
COPY app.db /app/app.db

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/app"]

