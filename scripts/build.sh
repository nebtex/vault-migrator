#!/usr/bin/env bash
set -e
go get github.com/mitchellh/gox
#pack static files
build_dir=$(pwd)
mkdir -p dist dist/bin
curl -fsL https://github.com/gliderlabs/sigil/releases/download/v0.4.0/sigil_0.4.0_Linux_x86_64.tgz  | tar -zxC /tmp
/tmp/sigil -p -f version.tmpl VAULT_MIGRATOR_RELEASE=$VAULT_MIGRATOR_RELEASE > version.go
gox -output dist/vault-migrator_{{.OS}}_{{.Arch}}
cd dist
dist=$(pwd)
mkdir -p prebuild release
for file in *
do
    if [[ -f $file ]]; then
        arch_dir="prebuild/${file%%.*}"
        if [ "${file#*.}" == "exe" ]; then
          binary="vault-migrator.${file#*.}"
        else
          binary="vault-migrator"
        fi
        mkdir -p "${arch_dir}"
        cp $file "${arch_dir}/${binary}"
        cp $build_dir/README.md "${arch_dir}/README.md"
        cp  $build_dir/LICENSE "${arch_dir}/LICENSE"
        cd "${arch_dir}"
        zip ${file%%.*}.zip README.md LICENSE ${binary}
        cp ${file%%.*}.zip ../../release
        tar -cvzf ${file%%.*}.tar.gz README.md LICENSE ${binary}
        cp ${file%%.*}.tar.gz ../../release
        cd $dist
    fi
done
go get -u github.com/tcnksm/ghr
if [ "${VAULT_MIGRATOR_RELEASE}" == "latest" ]; then
  ghr -u nebtex -replace "${VAULT_MIGRATOR_RELEASE}" release
else
  ghr -u nebtex -replace "v${VAULT_MIGRATOR_RELEASE}" release
fi

cd $build_dir

docker login -u $DOCKER_HUB_USER -p $DOCKER_HUB_PASSWORD

docker build -t nebtex/vault-migrator:$VAULT_MIGRATOR_RELEASE .
# upload to dockerhub
docker push nebtex/vault-migrator:$VAULT_MIGRATOR_RELEASE
