Custom gemini server configuration
=================================

```
doas pkg_add go
git clone https://github.com/a-h/gemini ~/git/gemini
cp -Rpv ./myserver ~/git/gemini/myserver
cd ~/git/gemini/myserver
go build
doas cp -p ./myserver /usr/local/bin/geminid
```
