.PHONY: all build run push clean

tag = v0.1
releaseName = pixiu-webshell
dockerhubUser = jacky06

all: run

run: build
	docker run -it --rm $(dockerhubUser)/$(releaseName):$(tag)

build:
	docker build -t $(dockerhubUser)/$(releaseName):$(tag) .

push: build
	docker push $(dockerhubUser)/$(releaseName):$(tag)

clean:
	rm -rf ./$(releaseName)
