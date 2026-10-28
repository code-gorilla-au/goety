#####################
##@ DB   
#####################

db-tables: ## List tables
	AWS_REGION=$(AWS_REGION) aws dynamodb list-tables --endpoint-url $(DYNAMODB_LOCAL_ENDPOINT)

db-seed: ## Start local DynamoDB and create/seed the e2e table via cmd/local
	docker-compose up -d
	go run $(PWD)/cmd/local/main.go

db-kill: ## Kill db
	docker-compose down
