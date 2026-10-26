#####################
##@ E2e Tests
#####################

e2e-dump: ## e2e dump
	AWS_REGION=$(AWS_REGION) \
	AWS_ACCESS_KEY_ID=$(AWS_ACCESS_KEY_ID) \
	AWS_SECRET_ACCESS_KEY=$(AWS_SECRET_ACCESS_KEY) \
	./goety dump -t="goety" -e="$(DYNAMODB_LOCAL_ENDPOINT)" --path="./e2e-dump-out.json"
