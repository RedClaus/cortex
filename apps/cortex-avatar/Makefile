.PHONY: build dev clean install

# Build the application
build:
	wails build

# Run in development mode
dev:
	wails dev

# Clean build artifacts
clean:
	rm -rf build/bin

# Install frontend dependencies
install:
	cd frontend && npm install

# Generate Wails bindings
generate:
	wails generate module
