#####################
##@ DB   
#####################

db-tables: ## List tables
	AWS_REGION=$(AWS_REGION) aws dynamodb list-tables --endpoint-url $(DYNAMODB_LOCAL_ENDPOINT)

db-seed: ## Create and seed table
	docker-compose up -d
	go run $(PWD)/cmd/local/main.go

db-kill: ## Kill db
	docker-compose down
