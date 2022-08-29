.PHONY: run build image push

tag = v0.1
releaseName = gopixiu
dockerhubUser = sl01248

ALL: push

run: build
	./gopixiu --configfile ./config.yaml
	
build:
	go build -o $(releaseName) ./cmd/

image:
	docker build -t $(dockerhubUser)/$(releaseName):$(tag) .

push: image
	docker push $(dockerhubUser)/$(releaseName):$(tag)
