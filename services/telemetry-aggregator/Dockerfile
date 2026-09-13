FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ballast-telemetry .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ballast-telemetry /ballast-telemetry
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/ballast-telemetry"]
