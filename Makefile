default: install

tidy:
	go mod tidy

# docs:
# 	go generate ./...

docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-name authress

install:
	go install .

test:
	go test -count=1 -parallel=4 ./...

integration:
	TF_ACC=1 AUTHRESS_KEY=1 go test -count=1 -parallel=4 -timeout 10m -v ./...
