SRCS := $(wildcard internal/**/*.go)
TSTS := $(wildcard tests/*)
CHK_TSTS := $(subst tests,.test,$(subst .go,,$(TSTS)))
CHK_TSTS_LL := $(addsuffix /main.ll,$(TSTS))

.PHONY: run test clean

run: prog.exe
	./prog.exe

# regression testing
test: $(CHK_TSTS)
	@echo tests completed

clean:
	@rm -rf .test $(CHK_TSTS_LL)
	@echo binary files cleaned up

$(CHK_TSTS): .test/%: tests/%/main.ll
	@mkdir -p $(dir $@)
	@echo "[[COMPILING TEST [llvm] $^]]"
	@llc-18 $^ -o - | clang -o $@ internal/gc/gc.c -x assembler -
	@echo "[[RUNNING TEST $^]]"
	@./$@ < $(dir $^)/in.txt | diff - $(dir $^)/out.txt

$(CHK_TSTS_LL): tests/%/main.ll: tests/%/main.go $(SRCS)
	@echo [[COMPILING TEST [gocomp] $<]]
	@cat $< | go run ./cmd/compiler | tee $(dir $<)/main.ll | opt-18 -S -o $(dir $<)/main-opt.ll

prog.exe: prog.s
	clang -o $@ internal/gc/gc.c $^

prog.s: prog.ll
	llc-18 $^
	opt-18 -S -o prog_opt.ll $^

prog.ll: prog.go $(SRCS)
	go run ./cmd/compiler $< > $@
