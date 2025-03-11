FROM public.ecr.aws/docker/library/golang:1.22.5-alpine3.20 AS base

ARG GITHUB_USER
ARG GITHUB_PASSWORD

ENV GOPRIVATE=github.com/ulascansenturk/*
ENV CGO_ENABLED=0
ENV GOOS=linux

# Precompile the standard library for faster builds
RUN go install -v -installsuffix cgo -a std

RUN apk update && apk add git make --no-cache

RUN go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.17.0

WORKDIR /app


COPY go.mod .
COPY go.sum .
RUN go mod download
# Precompile application dependencies for faster builds
RUN go mod graph | awk '{if ($1 !~ "@") print $2}' | xargs go get

COPY Makefile Makefile
COPY cmd cmd
COPY db db
COPY internal internal
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

FROM base AS dev


# Required for pg_dump
RUN apk update && apk add --no-cache postgresql-client
RUN apk update && apk add --no-cache gcc libc-dev bash make curl

RUN go install github.com/vektra/mockery/v2@v2.32.4
RUN go install github.com/deepmap/oapi-codegen/cmd/oapi-codegen@v1.16.2

FROM base AS build

RUN go mod tidy
RUN go build -o /server ./cmd/server/*.go

FROM public.ecr.aws/docker/library/alpine:3.19 AS app

COPY --from=build $GOROOT/go/bin/migrate /usr/local/bin/
COPY --from=build /usr/bin/make /usr/local/bin/

WORKDIR /app

COPY --from=build /app/Makefile ./

COPY --from=build /app/db/migrations /app/db/migrations
COPY --from=build /app/db/scripts/migrate.sh /app/db/scripts/migrate.sh

COPY --from=build /server /app/bin/server
