Custom gemini server configuration
=================================

```
doas pkg_add go
git clone https://github.com/a-h/gemini ~/git/gemini
cp -Rpv ./myserver ~/git/gemini/myserver
cd ~/git/gemini/myserver
go build main.go
doas cp -p ./main /usr/local/bin/geminid
```
