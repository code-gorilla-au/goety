#####################
##@ Lints   
#####################

lint: ## Lint tools
	go vet ./...
	golangci-lint run ./...

scan: ## run golang security scan
	govulncheck ./...

trivy: ## run trivy scan
	@trivy fs .