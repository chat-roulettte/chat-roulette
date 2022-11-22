FROM docker.io/library/golang:1.19 as build
ADD . /src
WORKDIR /src
RUN go build -o /src/bin/app-manifest cmd/app-manifest/main.go

# -------------------------------------------

FROM gcr.io/distroless/base
LABEL org.opencontainers.image.licenses=AGPLv3
LABEL org.opencontainers.image.source=https://github.com/chat-roulettte/chat-roulette
LABEL org.opencontainers.image.description="App Manifest installer tool"
COPY --from=build /src/bin/app-manifest /app-manifest
ADD . /chat-roulette
WORKDIR /chat-roulette
USER 10000:10000
ENTRYPOINT [ "/app-manifest" ]
