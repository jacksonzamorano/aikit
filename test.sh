#!/bin/bash
rm -rf tests
set -a
source .env
set +a
go test $@
