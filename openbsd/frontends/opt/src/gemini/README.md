Custom gemini server configuration
=================================

```
git clone https://github.com/a-h/gemini
cp -Rpv ./myserver ./gemini/myserver
cd ./gemini/myserver
go build main.go
doas cp -p ./main /usr/local/bin/geminid
```
