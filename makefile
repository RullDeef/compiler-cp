SRCS := $(wildcard internal/**/*.go)
TSTS := $(wildcard tests/*.go)
CHK_TSTS := $(subst tests,.test,$(subst .go,,$(TSTS)))

.PHONY: run test

run: prog.exe
	./prog.exe

# regression testing
test: $(CHK_TSTS)
	@echo tests completed

$(CHK_TSTS): .test/%: tests/%.go $(SRCS)
	@mkdir -p $(dir $@)
	@echo [[COMPILING TEST $<]]
	@cat $< | go run ./cmd/compiler | llc-18 | clang -o $@ -x assembler -
	@echo [[RUNNING TEST $<]]
	@./$@

prog.exe: prog.s
	clang -o $@ $^

prog.s: prog.ll
	llc-18 $^
	opt-18 -S -o prog_opt.ll $^

prog.ll: prog.go $(SRCS)
	go run ./cmd/compiler > $@
