deps:
	glide install

lint:
		gometalinter --disable-all --enable=dupl --enable=errcheck --enable=goconst \
    	--enable=golint --enable=gosimple --enable=ineffassign --enable=interfacer \
    	--enable=misspell --enable=staticcheck --enable=structcheck --enable=gocyclo \
    	--enable=unused --enable=vet --enable=vetshadow --enable=lll \
    	--line-length=160 --deadline=60s --vendor --dupl-threshold=100 ./...

create_test_services:
	docker run -d -e 'SKIP_SETCAP=yes' -e 'VAULT_DEV_ROOT_TOKEN_ID=myroot' -e 'VAULT_DEV_LISTEN_ADDRESS=0.0.0.0:8200' -p 8200:8200 vault

configure_vault:
	bash scripts/configure_vault.sh

test:
	bash scripts/test.sh

build:
	bash scripts/build.sh

bundle_react:
	bash scripts/bundle_react.sh

docs:
	bash scripts/build_and_publish_docs.sh

test_env: create_test_services  create_test_dbs

remove_test_env: remove_test_services

.PHONY: lint deps
