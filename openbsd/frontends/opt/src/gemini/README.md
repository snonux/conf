Custom gemini server configuration
=================================

```
doas pkg_add go
git clone https://github.com/a-h/gemini ~/git/gemini
mkdir ~/git/gemini/myserver
cp ./myserver/main.go ~/git/gemini/myserver/
cd ~/git/gemini/myserver
go build
doas sh -c 'rcctl stop geminid; sleep 1; cp -p ./myserver /usr/local/bin/geminid; rcctl start geminid'
```
