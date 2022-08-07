
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
	ls docs/**/* | entr -cr ./alvu --path='./docs'

pages: docs
	rm -rf alvu
	rm -rf .tmp
	mkdir .tmp
	mv dist/* .tmp
	git checkout pages  
	rm -rf ./*
	mv .tmp/* .
	git add -A; git commit -m "update pages"; git push origin pages;
	git checkout main
