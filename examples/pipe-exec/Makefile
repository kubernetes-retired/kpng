##
## Make for kpng-example
##
## Targets;
##  help - This printout
##  binaries - Build binaries
##  clean - Remove built files
##
##  Items are built in the "./_output/" directory
##


O ?= ./_output
BINARIES := $(O)/kpng-json $(O)/kpng-callout

$(O)/% : ./cmd/%
	CGO_ENABLED=0 go build -ldflags "-extldflags '-static'" -o $@ ./$<
	@strip $@

.PHONY: help binaries clean

help:
	@grep '^##' $(lastword $(MAKEFILE_LIST)) | cut -c3-

binaries: $(BINARIES)

clean:
	rm -rf ./_output

$(BINARIES): | $(O)

$(O):
	@mkdir -p $(O)

