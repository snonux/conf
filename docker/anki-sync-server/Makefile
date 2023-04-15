all:
	if [ ! -d anki-sync-server ]; then \
		git clone https://github.com/ankicommunity/anki-sync-server; \
	else \
		cd anki-sync-server && git pull && cd ..; \
  fi
	docker build -t anki-sync-server:latest . 
