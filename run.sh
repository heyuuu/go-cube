#/bin/bash
go generate . && go build -o tmp/cube && ./tmp/cube $@