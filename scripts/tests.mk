COVER_OUTPUT_RAW := coverage.out
COVER_OUTPUT_HTML := coverage.html

#####################
##@ Tests            
#####################

test: test-unit ## Run all tests

test-unit: ## Run unit tests
	go test -coverprofile $(COVER_OUTPUT_RAW) --short -cover  -failfast ./...

test-cover: ## generate html coverage report + open
	go tool cover -html=$(COVER_OUTPUT_RAW) -o $(COVER_OUTPUT_HTML)
	open coverage.html


test-integration: build ## Run integration tests
	./goety dump -e http://localhost:8000 -t "dev-main-adjacent" -p "./test.json"