CLANG ?= clang
STRIP ?= llvm-strip

# use uapi and bpf helper defs from libbpf to keep things consistent
LIBBPF_VER := v0.8.0
INCLUDES := -I./libbpf/include -I./libbpf/src
CFLAGS := -O2 -g -Wall -Werror $(INCLUDES)
INSTALL = install

.PHONY: clean bytecode

download-libbpf: 
	./hack/install-libbpf.sh $(LIBBPF_VER)

clean:
	-$(RM) *.o
	-$(RM) bpf_bp*.go
	-$(RM) -r libbpf

bytecode: export BPF_CLANG := $(CLANG)
bytecode: export BPF_CFLAGS := $(CFLAGS)
bytecode:
	./hack/install-libbpf.sh $(LIBBPF_VER)
	go generate -x ./
