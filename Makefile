.PHONY: build engine server web run test test-engine test-server test-web test-e2e format

CXXFLAGS ?= -O3
PYTHON ?= python3
CLANG_FORMAT ?= clang-format
ENGINE_SOURCES := $(filter-out engine/src/main.cpp,$(wildcard engine/src/*.cpp))
ENGINE_HEADERS := $(wildcard engine/include/xiangqi/*)
ENGINE_TESTS := $(patsubst engine/tests/%.cpp,build/engine/%,$(wildcard engine/tests/*.cpp))
ENGINE_FLAGS := -std=c++17 -Wall -Wextra -Werror -pedantic -pthread -Iengine/include

build: engine server web

engine: build/engine/xiangqi-engine

build/engine/xiangqi-engine: engine/src/main.cpp $(ENGINE_SOURCES) $(ENGINE_HEADERS)
	mkdir -p build/engine
	$(CXX) $(CXXFLAGS) $(ENGINE_FLAGS) engine/src/main.cpp $(ENGINE_SOURCES) -o $@

build/engine/%: engine/tests/%.cpp $(ENGINE_SOURCES) $(ENGINE_HEADERS)
	mkdir -p build/engine
	$(CXX) $(CXXFLAGS) $(ENGINE_FLAGS) $< $(ENGINE_SOURCES) -o $@

server:
	mkdir -p build
	cd server && GOWORK=off go build -o ../build/chess-server ./cmd/chess
	cd server && GOWORK=off go build -o ../build/chess-admin ./cmd/chess-admin

web:
	cd web && npm ci && npm run build

run:
	./build/chess-server -root . -addr 127.0.0.1:8765

test: test-engine test-server test-web

test-engine: engine $(ENGINE_TESTS)
	build/engine/analysis_native_test
	build/engine/engine_contract_test
	build/engine/search_components_test
	$(PYTHON) -m unittest discover -s tests -v

test-server: engine
	cd server && GOWORK=off go test -race ./... && GOWORK=off go vet ./...

test-web:
	cd web && npm test

test-e2e:
	cd web && npm run test:e2e

format:
	find engine -type f \( -name '*.cpp' -o -name '*.hpp' \) -exec $(CLANG_FORMAT) -i --style=file {} +
