# full pkg name
PKG = github.com/G-Node/gin-doi

# Binary
APP = gindoid

# Build loc
BUILDLOC = build

# Install location
INSTLOC = $(GOPATH)/bin

cwd = $(shell pwd)

# Build flags
VERNUM = $(shell cut -d= -f2 version)
ncommits = $(shell git rev-list --count HEAD)
BUILDNUM = $(shell printf '%06d' $(ncommits))
COMMITHASH = $(shell git rev-parse HEAD)
LDFLAGS = -ldflags="-X main.appversion=$(VERNUM) -X main.build=$(BUILDNUM) -X main.commit=$(COMMITHASH)"

SOURCES = $(shell find . -type f -iname "*.go") version

.PHONY: $(APP) install clean uninstall

$(APP): $(BUILDLOC)/$(APP)

install: $(APP)
	install $(BUILDLOC)/$(APP) $(INSTLOC)/$(APP)

clean:
	rm -r $(BUILDLOC)

uninstall:
	rm $(INSTLOC)/$(APP)

$(BUILDLOC)/$(APP): $(SOURCES)
	go build -trimpath $(LDFLAGS) $(GCFLAGS) -o $(BUILDLOC)/$(APP) ./cmd/gindoid
