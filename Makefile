# NetteFatura Makefile

# Load environment variables
include .env
export

# Default target
.PHONY: help
help:
	@echo "Available commands:"
	@echo "  make env          - Load environment variables from .env"
	@echo "  make test         - Run all tests"
	@echo "  make test-verbose - Run all tests with verbose output"
	@echo ""
	@echo "Location tests:"
	@echo "  make test-city    - Test GetCityID function"
	@echo "  make test-district - Test GetDistrictID function"
	@echo "  make test-district-merkez - Test GetDistrictID_Merkez function"
	@echo "  make test-district-by-names - Test GetDistrictIDByNames function"
	@echo "  make test-district-by-names-merkez - Test GetDistrictIDByNames_Merkez function"
	@echo "  make test-city-name - Test GetCityName function"
	@echo "  make test-district-name - Test GetDistrictName function"
	@echo ""
	@echo "Recipient tests:"
	@echo "  make test-recipient-list - Test GetRecipientList function"
	@echo "  make test-recipient-detail - Test GetRecipientDetail function"
	@echo "  make test-customer-or-existing - Test CreateCustomerOrGetExisting function"

# Load environment variables
.PHONY: env
env:
	@echo "Environment variables loaded from .env"
	@echo "NETTEFATURA_VKN=$$NETTEFATURA_VKN"
	@echo "NETTEFATURA_COMPANY_ID=$$NETTEFATURA_COMPANY_ID"

# Run all tests
.PHONY: test
test:
	go test ./...

# Run all tests with verbose output
.PHONY: test-verbose
test-verbose:
	go test -v ./...

# Location tests
.PHONY: test-city
test-city:
	go test -v -run TestGetCityID

.PHONY: test-district
test-district:
	go test -v -run "TestGetDistrictID$$"

.PHONY: test-district-merkez
test-district-merkez:
	go test -v -run TestGetDistrictID_Merkez

.PHONY: test-district-by-names
test-district-by-names:
	go test -v -run "TestGetDistrictIDByNames$$"

.PHONY: test-district-by-names-merkez
test-district-by-names-merkez:
	go test -v -run TestGetDistrictIDByNames_Merkez

.PHONY: test-city-name
test-city-name:
	go test -v -run TestGetCityName

.PHONY: test-district-name
test-district-name:
	go test -v -run TestGetDistrictName

# Recipient tests
.PHONY: test-recipient-list
test-recipient-list:
	go test -v -run TestGetRecipientList

.PHONY: test-recipient-detail
test-recipient-detail:
	go test -v -run TestGetRecipientDetail

.PHONY: test-customer-or-existing
test-customer-or-existing:
	go test -v -run TestCreateCustomerOrGetExisting

# Clean
.PHONY: clean
clean:
	go clean
	rm -f coverage.out

# Coverage
.PHONY: coverage
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out