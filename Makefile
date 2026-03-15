# dehub-server Makefile

.PHONY: test coverage coverage-html build clean lint

# 运行测试
test:
	go test -v -race ./...

# 运行测试并检查覆盖率
coverage:
	@echo "运行测试..."
	@go test -race -coverprofile=coverage.out ./... 2>&1 | grep -E "PASS|FAIL|ok|coverage:"
	@echo ""
	@python3 check_coverage.py --threshold 95.0

# 生成 HTML 覆盖率报告
coverage-html:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "覆盖率报告已生成: coverage.html"

# 代码检查
lint:
	@which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run ./...

# 构建
build:
	go build -o dehub-server .

# 清理
clean:
	rm -f dehub-server coverage.out coverage.html
