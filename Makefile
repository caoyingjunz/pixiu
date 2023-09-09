.PHONY: run build image push clean

tag = v0.1
releaseName = pixiu
dockerhubUser = jacky06

ALL: run

run: build
	./pixiu --configfile ./config.yaml

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
	swag init --generalInfo ./cmd/pixiuserver.go --output ./api/docs
