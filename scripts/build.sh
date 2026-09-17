#!/usr/bin/env bash

[[ ! -d "cmd" ]] && echo "run from project root" && exit 1

mkdir -p bin
go build -o bin/morgul ./cmd/morgul
echo ">> ./bin/morgul"

