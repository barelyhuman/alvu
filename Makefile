
.PHONY.: clean build


clean:
	rm -rf alvu 

build: 
	go build -ldflags '-s -w'
	
demo: 
	go run . --path lab
