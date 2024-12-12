.PHONY: run build image push clean

tag = v0.1
releaseName = pixiu
dockerhubUser ?= jacky06
k8sVersion ?= v1.23.6
helmVersion ?= v3.7.1
targetDir ?= dist
commitHash = $(shell git rev-parse --short HEAD)
# e.g. 1862ce5-20240203180617
version = $(commitHash)-$(shell date +%Y%m%d%H%M%S)

ALL: run

run: build
	./pixiu --configfile ./config.yaml

build:
	go build -o $(targetDir)/$(releaseName) -ldflags "-X 'main.version=$(version)'" ./cmd/

image:
	docker build -t $(dockerhubUser)/$(releaseName):$(tag) --build-arg VERSION=$(version) .

push: image
	docker push $(dockerhubUser)/$(releaseName):$(tag)

webshell-image:
	docker build --build-arg K8S_VERSION=$(k8sVersion) \
		--build-arg HELM_VERSION=$(helmVersion) \
		-t $(dockerhubUser)/pixiu-webshell:$(tag) -f docker/Dockerfile .

push-webshell-image: webshell-image
	docker push $(dockerhubUser)/pixiu-webshell:$(tag)

licfmt:
	go run hack/tools/licfmt/licfmt.go -v ./*

clean:
	-rm -f ./$(releaseName)

.PHONY: api-docs
api-docs: ## generate the api docs
	swag init --generalInfo ./cmd/pixiuserver.go --output ./api/docs
