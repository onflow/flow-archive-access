FROM golang:1.18-buster AS build-setup

RUN apt-get update \
 && apt-get -y install cmake zip sudo git

# Build flow-go so crypto doesn't break compilation

ENV FLOW_GO_REPO="https://github.com/onflow/flow-go"
ENV FLOW_GO_BRANCH=v0.28.5

RUN mkdir /archive /docker /flow-go

WORKDIR /archive

# clone repos
COPY . /archive
RUN git clone --branch $FLOW_GO_BRANCH $FLOW_GO_REPO /flow-go

RUN ln -s /flow-go /archive/flow-go

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build  \
    make -C /flow-go crypto_setup_gopath #prebuild crypto dependency \
    bash crypto_setup.sh

RUN ls -la /flow-go/crypto/


FROM build-setup AS build-binary

WORKDIR /archive

RUN	--mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build  \
    CGO_ENABLED=1 GOOS=linux go build -o /app --tags "relic,netgo" -ldflags "-extldflags -static" ./cmd/archive-access-api && \
    chmod a+x /app

## Add the statically linked binary to a distroless image
FROM gcr.io/distroless/base-debian11  AS production

COPY --from=build-binary /app /app

EXPOSE 5006

ENTRYPOINT ["/app"]