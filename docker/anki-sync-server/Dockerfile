# Could not use Alpine due to wrong Python versions available.
# Well, disk space is cheap, nowadays!
FROM rockylinux/rockylinux

RUN dnf update -y
RUN dnf install -y python39 

ADD ./anki-sync-server /anki-sync-server
RUN adduser anki-sync-server
WORKDIR /anki-sync-server/src

RUN pip3.9 install -r /anki-sync-server/src/requirements.txt
RUN pip3.9 install -e /anki-sync-server/src

CMD ["python3.9", "-m", "ankisyncd"]
