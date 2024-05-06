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
	AWS_PROFILE=$(AWS_PROFILE) ./goety purge -e $(DYNAMODB_LOCAL_ENDPOINT) -t $(TEST_TABLE_NAME) -p $(TEST_PRIMARY_KEY) -s $(TEST_SORT_KEY)
test-seed: build ## Run seed integration tests
	./goety seed -e $(DYNAMODB_LOCAL_ENDPOINT) -t $(TEST_TABLE_NAME) -f $(PWD)/$(TEST_JSON_OUT_FILE)

test-dump: build ## Run dump integration tests
	AWS_PROFILE=$(AWS_PROFILE) ./goety dump  -t $(TEST_TABLE_NAME) --path $(TEST_JSON_OUT_FILE) -f "contains(#pk, :pk)" -N "#pk = pk" -V ":pk = AGREEMENT"