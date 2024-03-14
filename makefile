.PHONY: run

run: prog.exe
	./prog.exe

prog.exe: prog.s
	clang -o $@ $^

prog.s: prog.ll
	llc-18 $^

prog.ll: prog.go
	go run ./cmd/compiler > $@
