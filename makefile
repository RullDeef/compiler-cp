SRCS := $(wildcard internal/**/*.go)

.PHONY: run

run: prog.exe
	./prog.exe

prog.exe: prog.s
	clang -o $@ $^

prog.s: prog.ll
	llc-18 $^
	opt-18 -S -o prog_opt.ll $^

prog.ll: prog.go $(SRCS)
	go run ./cmd/compiler > $@
