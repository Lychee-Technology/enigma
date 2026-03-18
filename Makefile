DOCKER_IMAGE := lychee.technology/enigma:latest

docker-build:
	docker build -t $(DOCKER_IMAGE) -f Dockerfile .

