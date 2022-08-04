
.PHONY.: all

all: clean build

clean:
	rm -rf alvu 

build: 
	go build -ldflags '-s -w'
	
demo: 
	go run . --path lab

docs: build 
	./alvu --path="docs" --baseurl="/alvu/"

docs_dev: build 
	./alvu --path="docs"