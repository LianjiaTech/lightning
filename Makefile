# colors compatible setting
CRED:=$(shell tput setaf 1 2>/dev/null)
CGREEN:=$(shell tput setaf 2 2>/dev/null)
CYELLOW:=$(shell tput setaf 3 2>/dev/null)
CEND:=$(shell tput sgr0 2>/dev/null)

# Add mysql version for testing `MYSQL_RELEASE=percona MYSQL_VERSION=5.7 make docker`
# MySQL 5.1 `MYSQL_RELEASE=vsamov/mysql-5.1.73 make docker`
# MYSQL_RELEASE: mysql, percona, mariadb ...
# MYSQL_VERSION: latest, 8.0, 5.7, 5.6, 5.5 ...
# use mysql:latest as default
MYSQL_RELEASE := $(or ${MYSQL_RELEASE}, ${MYSQL_RELEASE}, mysql)
MYSQL_VERSION := $(or ${MYSQL_VERSION}, ${MYSQL_VERSION}, latest)

# Build the project
.PHONY: build
build:
	@echo "$(CGREEN)Building ...$(CEND)"
	@bash ./genver.sh
	@ret=0 && for d in $$(go list -f '{{if (eq .Name "main")}}{{.ImportPath}}{{end}}' ./...); do \
		b=$$(basename $${d}) ; \
		go build ${GCFLAGS} -o $${b} $$d || ret=$$? ; \
	done ; exit $$ret
	@echo "build Success!"

.PHONY: release
release: build
	@echo "$(CGREEN)Cross platform building for release ...$(CEND)"
	@mkdir -p release
	@for GOOS in darwin linux; do \
		for GOARCH in amd64; do \
			for d in $$(go list -f '{{if (eq .Name "main")}}{{.ImportPath}}{{end}}' ./...); do \
				b=$$(basename $${d}) ; \
				echo "Building $${b}.$${GOOS}-$${GOARCH} ..."; \
				GOOS=$${GOOS} GOARCH=$${GOARCH} go build ${GCFLAGS} ${LDFLAGS} -v -o release/$${b}.$${GOOS}-$${GOARCH} $$d 2>/dev/null ; \
			done ; \
		done ;\
	done

# Code format
.PHONY: fmt
fmt:
	@echo "$(CGREEN)Run gofmt on all source files ...$(CEND)"
	@echo "gofmt -l -s -w ..."
	@ret=0 && for d in $$(go list -f '{{.Dir}}' ./... | grep -v /vendor/); do \
  	gofmt -l -s -w $$d/*.go || ret=$$? ; \
	done ; exit $$ret

# Run golang test cases
.PHONY: test
test:
	@echo "$(CGREEN)Run all test cases ...$(CEND)"
	go test -timeout 10m -race ./...
	@echo "test Success!"

# Code Coverage
# colorful coverage numerical >=90% GREEN, <80% RED, Other YELLOW
.PHONY: cover
cover: test
	@echo "$(CGREEN)Run test cover check ...$(CEND)"
	go test -coverpkg=./... -coverprofile=coverage.data ./... | column -t
	go tool cover -html=coverage.data -o coverage.html
	go tool cover -func=coverage.data -o coverage.txt
	@tail -n 1 coverage.txt | awk '{sub(/%/, "", $$NF); \
	if($$NF < 80) \
		{print "$(CRED)"$$0"%$(CEND)"} \
	else if ($$NF >= 90) \
		{print "$(CGREEN)"$$0"%$(CEND)"} \
	else \
		{print "$(CYELLOW)"$$0"%$(CEND)"}}'


# Update tidb vendor
.PHONY: tidb
tidb:
	@echo "$(CGREEN)Update tidb deps ...$(CEND)"
	govendor fetch -v github.com/pingcap/tidb/...

# make pingcap parser
.PHONY: pingcap-parser
pingcap-parser: tidb
	@echo "$(CGREEN)Update pingcap parser deps ...$(CEND)"
	govendor fetch -v github.com/pingcap/parser/...

# Update all vendor
.PHONY: vendor
vendor: pingcap-parser

.PHONY: docker
docker:
	@echo "$(CGREEN)Build mysql test environment ...$(CEND)"
	@docker stop lightning-mysql 2>/dev/null || true
	@docker wait lightning-mysql 2>/dev/null >/dev/null || true
	@echo "docker run --name lightning-mysql $(MYSQL_RELEASE):$(MYSQL_VERSION)"
	@docker run --name lightning-mysql --rm -d \
	-e MYSQL_ROOT_PASSWORD='******' \
	-e MYSQL_DATABASE=test \
	-p 3306:3306 \
	-v `pwd`/test/schema.sql:/docker-entrypoint-initdb.d/schema.sql \
	-v `pwd`/test/init.sql:/docker-entrypoint-initdb.d/init.sql \
	$(MYSQL_RELEASE):$(MYSQL_VERSION)

	@echo "waiting for test database initializing "
	@timeout=180; while [ $${timeout} -gt 0 ] ; do \
		if ! docker exec lightning-mysql mysql --user=root --password='******' --host "127.0.0.1" --silent -NBe "do 1" >/dev/null 2>&1 ; then \
			timeout=`expr $$timeout - 1`; \
			printf '.' ;  sleep 1 ; \
		else \
			echo "." ; echo "mysql test environment is ready!" ; break ; \
		fi ; \
		if [ $$timeout = 0 ] ; then \
			echo "." ; echo "$(CRED)docker lightning-mysql start timeout(180 s)!$(CEND)" ; exit 1 ; \
		fi ; \
	done

.PHONY: docker-connect
docker-connect:
	@docker exec -it lightning-mysql mysql --user=root --password='******' --host "127.0.0.1" test

# attach docker container with bash interactive mode
.PHONY: docker-it
docker-it:
	docker exec -it lightning-mysql /bin/bash

# Installs our project: copies binaries
install: build
	@echo "$(CGREEN)Install ...$(CEND)"
	go install ./...
	@echo "install Success!"
