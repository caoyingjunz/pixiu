.PHONY: run build image push clean

tag = v0.1
releaseName = gopixiu
dockerhubUser = jacky06

ALL: run

run: build
	./gopixiu --configfile ./config.yaml

build:
	go build -o $(releaseName) ./cmd/

image:
	docker build -t $(dockerhubUser)/$(releaseName):$(tag) .

push: image
	docker push $(dockerhubUser)/$(releaseName):$(tag)

clean:
	-rm -f ./$(releaseName)

.PHONY: api-docs
api-docs: ## generate the api docs
	swag init --generalInfo ./cmd/main.go --output ./api/docs
