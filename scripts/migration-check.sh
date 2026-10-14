#!/bin/sh
set -eu
go run ./cmd/migrate down
go run ./cmd/migrate up
go run ./cmd/migrate down
go run ./cmd/migrate up
