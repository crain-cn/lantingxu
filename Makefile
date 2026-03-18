# 兰亭序 · 故事续写
BINARY := lantingxu
PORT   ?= 3000
DB_PATH ?= lantingxu.db

.PHONY: build clean test help db-reset

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(BINARY) .

clean:
	rm -f $(BINARY) $(DB_PATH)

test:
	go test -v ./...

help:
	@echo "  run     - 启动服务 (go run .)，PORT=$(PORT)"
	@echo "  build   - 编译为 $(BINARY)"
	@echo "  clean   - 删除二进制与数据库"
	@echo "  test    - 运行测试"
	@echo "  db-reset - 删除数据库，下次 run 时会重新建表并填充示例数据"

db-reset:
	rm -f $(DB_PATH)
