build: ./build
	go build -o ./build/gogeta

run: build ./build/gogeta
	./build/gogeta -h

release: build
	go build -buildmode=c-shared -o ./build/gogeta.so .

clean:
	rm -rf ./build/*

.PHONY: build release clean run