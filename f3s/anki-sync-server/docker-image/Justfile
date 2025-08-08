all:
	docker build -t anki-sync-server:25.07.5b --build-arg ANKI_VERSION=25.07.5 .
f3s:	
	docker build -t anki-sync-server:25.07.5b --build-arg ANKI_VERSION=25.07.5 .
	docker tag anki-sync-server:25.07.5b r0.lan.buetow.org:30001/anki-sync-server:25.07.5b
	docker push r0.lan.buetow.org:30001/anki-sync-server:25.07.5b
