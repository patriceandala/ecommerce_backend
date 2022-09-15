FROM golang:1.18-alpine as builder

# install tzdata to be copied on final image
RUN apk --no-cache add \
	tzdata \
	git \
	ca-certificates

WORKDIR /src
COPY go.mod go.sum ./

# download server dependencies
ENV GOPRIVATE "github.com/dropezy"
ARG GITHUB_TOKEN
RUN git config --global url."https://dropezy:${GITHUB_TOKEN}@github.com".insteadOf "https://github.com"
RUN go mod download

# build ems-api
COPY . .
ARG VERSION="latest"
RUN CGO_ENABLED=0 go build -ldflags="-w -s -X main.version=${VERSION}" -o ./dist/ems-api .

FROM scratch

# we need tzdata to load time location in Go program
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
# we need public certs to connect to mongo atlas
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# copy the binary that already built and all required files
COPY --from=builder /src/dist /
COPY --from=builder /src/env/sample.config /env/config

ENTRYPOINT ["/ems-api"]
