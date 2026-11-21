API_PORT ?= 8080
USERS ?= 10
RAMP_SECONDS ?= 10
DURATION_SECONDS ?= 120
REPORT_DIR ?= report-$(USERS)

.PHONY: build run test compose-up compose-down jmeter

build:
	go build -o api ./cmd/api

run:
	go run ./cmd/api

test:
	go test ./...

compose-up:
	docker compose up --build -d

compose-down:
	docker compose down

jmeter:
	jmeter -n -t tests/load/orders.jmx -l results-$(USERS).jtl -e -o $(REPORT_DIR) -JAPI_PORT=$(API_PORT) -JUSERS=$(USERS) -JRAMP_SECONDS=$(RAMP_SECONDS) -JDURATION_SECONDS=$(DURATION_SECONDS)
