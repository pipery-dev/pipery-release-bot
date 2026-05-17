FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/pipery-release-bot ./cmd/pipery-release-bot

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/pipery-release-bot /pipery-release-bot
EXPOSE 8080
ENTRYPOINT ["/pipery-release-bot"]
