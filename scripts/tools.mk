#####################
##@ Tools            
#####################

tools-all: tools-dev tools-scan ## Get all tools for development

tools-scan: ## get all the tools required
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest


tools-dev: ## Dev specific tooling
	go install github.com/matryer/moq@latest
	go install github.com/mitranim/gow@latest