APP_NAME = go-doc-generator
BUILD_DIR = bin

.PHONY: run build clean

run:
	go run ./cmd/.

build:
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/.

start: build
	./$(BUILD_DIR)/$(APP_NAME)

clean:
	rm -rf $(BUILD_DIR)
