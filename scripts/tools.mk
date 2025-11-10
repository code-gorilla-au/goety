#####################
##@ Tools            
#####################

tools-all: tools-dev tools-scan ## Get all tools for development

tools-scan: ## get all the tools required
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.2.4


tools-dev: ## Dev specific tooling
	go install github.com/matryer/moq@latest
	go install github.com/mitranim/gow@latest