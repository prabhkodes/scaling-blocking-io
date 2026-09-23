FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN CGO_ENABLED=0 go build -o /mock-third-party .

FROM gcr.io/distroless/static-debian12
COPY --from=build /mock-third-party /mock-third-party
EXPOSE 9000
ENTRYPOINT ["/mock-third-party"]
