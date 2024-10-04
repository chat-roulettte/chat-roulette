FROM docker.io/library/golang:1.22 as build
ADD . /src
WORKDIR /src
RUN make go/build

# -------------------------------------------

FROM gcr.io/distroless/base
LABEL org.opencontainers.image.licenses=AGPLv3
LABEL org.opencontainers.image.source=https://github.com/chat-roulettte/chat-roulette
LABEL org.opencontainers.image.description="Chat Roulette for Slack"
COPY --from=build /src/bin/chat-roulette /chat-roulette
COPY --from=build /src/docs/examples/config.example.json /etc/config.json
USER 10000:10000
EXPOSE 8080
ENTRYPOINT [ "/chat-roulette", "-c", "/etc/config.json", "--migrate" ]
