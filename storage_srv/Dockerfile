FROM golang:1.25 AS build
WORKDIR /src
COPY contracts ./contracts
COPY storage_srv ./storage_srv
WORKDIR /src/storage_srv
RUN go build -o /out/service .
FROM gcr.io/distroless/base-debian12
COPY --from=build /out/service /service
ENTRYPOINT ["/service"]

