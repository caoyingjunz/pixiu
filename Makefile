.PHONY: run build image push clean

tag = v0.1
releaseName = gopixiu
dockerhubUser = sl01248

ALL: run

run: build
	./gopixiu --configfile ./config.yaml --kubeconfig ./kubeconfig

build:
	go build -o $(releaseName) ./cmd/

image:
	docker build -t $(dockerhubUser)/$(releaseName):$(tag) .

push: image
	docker push $(dockerhubUser)/$(releaseName):$(tag)

clean:
	-rm -f ./$(releaseName)