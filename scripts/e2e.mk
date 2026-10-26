#####################
##@ E2e Tests
#####################

E2E_ITEM_COUNT := 1000

e2e: e2e-clean db-seed e2e-dump e2e-purge e2e-seed ## Run the full end to end harness
	@echo "e2e complete: dump, purge and seed verified"

e2e-clean: ## Remove e2e artifacts
	rm -f e2e-*.json

e2e-dump: build ## Dump the seeded table and assert the item count
	AWS_REGION=$(AWS_REGION) \
	AWS_ACCESS_KEY_ID=$(AWS_ACCESS_KEY_ID) \
	AWS_SECRET_ACCESS_KEY=$(AWS_SECRET_ACCESS_KEY) \
	./goety dump -t="$(TEST_TABLE_NAME)" -e="$(DYNAMODB_LOCAL_ENDPOINT)" --path="./e2e-dump.json"
	@test "$$(jq 'length' e2e-dump.json)" -eq $(E2E_ITEM_COUNT) || { echo "e2e dump: expected $(E2E_ITEM_COUNT) items" >&2; exit 1; }
	@echo "e2e dump: wrote $(E2E_ITEM_COUNT) items to e2e-dump.json"

e2e-purge: build ## Purge the table and assert it is empty
	AWS_REGION=$(AWS_REGION) \
	AWS_ACCESS_KEY_ID=$(AWS_ACCESS_KEY_ID) \
	AWS_SECRET_ACCESS_KEY=$(AWS_SECRET_ACCESS_KEY) \
	./goety purge -t="$(TEST_TABLE_NAME)" -e="$(DYNAMODB_LOCAL_ENDPOINT)" -p="$(TEST_PRIMARY_KEY)" -s="$(TEST_SORT_KEY)"
	AWS_REGION=$(AWS_REGION) \
	AWS_ACCESS_KEY_ID=$(AWS_ACCESS_KEY_ID) \
	AWS_SECRET_ACCESS_KEY=$(AWS_SECRET_ACCESS_KEY) \
	./goety dump -t="$(TEST_TABLE_NAME)" -e="$(DYNAMODB_LOCAL_ENDPOINT)" --path="./e2e-purge.json"
	@test "$$(jq 'length' e2e-purge.json)" -eq 0 || { echo "e2e purge: expected 0 items after purge" >&2; exit 1; }
	@echo "e2e purge: table $(TEST_TABLE_NAME) is empty"

e2e-seed: build e2e-dump ## Seed the table from the dump and assert the round trip
	AWS_REGION=$(AWS_REGION) \
	AWS_ACCESS_KEY_ID=$(AWS_ACCESS_KEY_ID) \
	AWS_SECRET_ACCESS_KEY=$(AWS_SECRET_ACCESS_KEY) \
	./goety seed -t="$(TEST_TABLE_NAME)" -e="$(DYNAMODB_LOCAL_ENDPOINT)" -f="./e2e-dump.json"
	AWS_REGION=$(AWS_REGION) \
	AWS_ACCESS_KEY_ID=$(AWS_ACCESS_KEY_ID) \
	AWS_SECRET_ACCESS_KEY=$(AWS_SECRET_ACCESS_KEY) \
	./goety dump -t="$(TEST_TABLE_NAME)" -e="$(DYNAMODB_LOCAL_ENDPOINT)" --path="./e2e-seed.json"
	@test "$$(jq 'length' e2e-seed.json)" -eq $(E2E_ITEM_COUNT) || { echo "e2e seed: expected $(E2E_ITEM_COUNT) items after seed" >&2; exit 1; }
	@jq -S 'sort_by(.$(TEST_PRIMARY_KEY), .$(TEST_SORT_KEY))' e2e-dump.json > e2e-expected.json
	@jq -S 'sort_by(.$(TEST_PRIMARY_KEY), .$(TEST_SORT_KEY))' e2e-seed.json > e2e-actual.json
	@diff -u e2e-expected.json e2e-actual.json
	@echo "e2e seed: re-seeded $(E2E_ITEM_COUNT) items and verified the round trip"

.PHONY: e2e e2e-clean e2e-dump e2e-purge e2e-seed
