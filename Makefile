BUILDDIR = build
GOFLAGS = -ldflags="-s -w"

APPS = executor scheduler
all: $(APPS)

$(BUILDDIR)/yamf-executor: $(wildcard apps/executor/*.go executor/*.go internal/*/*.go)
$(BUILDDIR)/yamf-scheduler: $(wildcard apps/scheduler/*.go scheduler/*.go internal/*/*.go)

$(BUILDDIR)/yamf-%:
	@mkdir -p $(dir $@)
	go build ${GOFLAGS} -o $@ ./apps/$*

$(APPS): %: $(BUILDDIR)/yamf-%

clean:
	rm -fr $(BUILDDIR)/yamf-*

.PHONY: clean all
.PHONY: $(APPS)
