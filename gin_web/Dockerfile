FROM golang:1.25 AS build
WORKDIR /src
COPY contracts ./contracts
COPY gin_web ./gin_web
WORKDIR /src/gin_web
RUN go build -o /out/service .
FROM gcr.io/distroless/base-debian12
COPY --from=build /out/service /service
ENTRYPOINT ["/service"]

