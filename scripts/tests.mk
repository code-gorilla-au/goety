COVER_OUTPUT_RAW := coverage.out
COVER_OUTPUT_HTML := coverage.html
TEST_JSON_OUT_FILE := test.json

#####################
##@ Tests            
#####################

test: test-unit ## Run all tests

test-unit: ## Run unit tests
	go test -coverprofile $(COVER_OUTPUT_RAW) --short -cover  -failfast ./...

test-cover: ## generate html coverage report + open
	go tool cover -html=$(COVER_OUTPUT_RAW) -o $(COVER_OUTPUT_HTML)
	open coverage.html

test-purge: build ## Run purge integration tests
	./goety purge -e $(DYNAMODB_LOCAL_ENDPOINT) -t $(TEST_TABLE_NAME) -p $(TEST_PRIMARY_KEY) -s $(TEST_SORT_KEY)

test-dump: build ## Run dump integration tests
	./goety dump -e $(DYNAMODB_LOCAL_ENDPOINT) -t $(TEST_TABLE_NAME) -p $(TEST_JSON_OUT_FILE)