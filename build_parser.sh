#!/bin/sh

cd internal/parser

docker run --rm -u $(id -u ${USER}):$(id -g ${USER}) -v `pwd`:/work any0ne22/antlr4:latest java -Xmx500M -cp /usr/local/lib/antlr4-tool.jar org.antlr.v4.Tool -Dlanguage=Go -visitor GoLexer.g4 GoParser.g4
