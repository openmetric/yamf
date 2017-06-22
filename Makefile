BLDDIR = build
GOFLAGS =

APPS = executor scheduler
all: $(APPS)

$(BUILDIR)/yamf-executor: $(wildcard apps/executor/*.go executor/*.go internal/*/*.go)
$(BULDDIR)/yamf-scheduler: $(wildcard apps/scheduler/*.go scheduler/*.go internal/*/*.go)

$(BLDDIR)/yamf-%:
	@mkdir -p $(dir $@)
	go build ${GOFLAGS} -o $@ ./apps/$*

$(APPS): %: $(BLDDIR)/yamf-%

clean:
	rm -fr $(BLDDIR)

.PHONY: clean all
.PHONY: $(APPS)
