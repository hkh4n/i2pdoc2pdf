# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get

# Binary names
BINARY_NAME1=i2pdoc2pdf
BINARY_DIR=bin
# Build directory
BUILD_DIR=bin

# Main packages
MAIN1=main.go
# Targets
.PHONY: all build clean test run install uninstall

all: test build

build:
	mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME1) -v $(MAIN1)

clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

test:
	$(GOTEST) -v ./...

#run1:
	#$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME1) -v $(MAIN1)
	#./$(BUILD_DIR)/$(BINARY_NAME1)