SHELL = /bin/bash
export GO111MODULE=on

NAME = geoipupdate
OUTPUT_DIR = bin

.PHONY: clean updater

clean:
	@rm -rf ${OUTPUT_DIR}

updater: clean
	@mkdir -p ${OUTPUT_DIR}
	go build -v -ldflags="-s -w" -o ${OUTPUT_DIR}/${NAME} ./cmd/${NAME}/...

docker-image: clean
	docker build -t ${NAME} -f Dockerfile .
