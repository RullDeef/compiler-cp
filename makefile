SRCS := $(wildcard internal/**/*.go)
TSTS := $(wildcard tests/*)
CHK_TSTS := $(subst tests,.test,$(subst .go,,$(TSTS)))

.PHONY: run test

run: prog.exe
	./prog.exe

# regression testing
test: $(CHK_TSTS)
	@echo tests completed

$(CHK_TSTS): .test/%: tests/%/main.go $(SRCS)
	@mkdir -p $(dir $@)
	@echo [[COMPILING TEST $<]]
	@cat $< | go run ./cmd/compiler | tee $(dir $<)/main.ll | llc-18 | tee $(dir $<)/main-opt.ll | clang -o $@ -x assembler -
	@echo [[RUNNING TEST $<]]
	@./$@ < $(basename $(dir $<))/in.txt | diff - $(basename $(dir $<))/out.txt

prog.exe: prog.s
	clang -o $@ $^

prog.s: prog.ll
	llc-18 $^
	opt-18 -S -o prog_opt.ll $^

prog.ll: prog.go $(SRCS)
	go run ./cmd/compiler $< > $@
