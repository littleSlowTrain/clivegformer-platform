FROM golang:1.25 AS build
WORKDIR /src
COPY contracts ./contracts
COPY file_srv ./file_srv
WORKDIR /src/file_srv
RUN go build -o /out/service .
FROM gcr.io/distroless/base-debian12
COPY --from=build /out/service /service
ENTRYPOINT ["/service"]

