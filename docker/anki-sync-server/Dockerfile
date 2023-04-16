# Could not use Alpine due to wrong Python versions available.
# Well, disk space is cheap, nowadays!
FROM rockylinux/rockylinux

RUN dnf update -y
RUN dnf install -y python39 

ADD ./anki-sync-server/src /anki-sync-server
WORKDIR /anki-sync-server

RUN pip3.9 install -r /anki-sync-server/requirements.txt
RUN pip3.9 install -e /anki-sync-server
RUN sed -i 's|^data_root =.*|data_root = /data/collections|; s|^auth_db_path =.*|auth_db_path = /data/auth.db|; s|^session_db_path =.*|session_db_path = /data/session.db|' /anki-sync-server/ankisyncd.conf 

RUN adduser anki-sync-server
USER anki-sync-server

CMD ["python3.9", "-m", "ankisyncd"]
