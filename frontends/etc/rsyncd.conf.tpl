<% my $allow = '*.buetow.org,localhost'; %>
max connections = 5
timeout = 300

[publicgemini]
comment = Public Gemini capsule content
path = /var/gemini
read only = yes
list = yes
uid = www
gid = www
hosts allow = <%= $allow %>

[publichttp]
comment = Public HTTP content
path = /var/www/htdocs
read only = yes
list = yes
uid = www
gid = www
hosts allow = <%= $allow %>

[sslcerts]
comment = TLS certificates
path = /etc/ssl
read only = yes
list = yes
hosts allow = <%= $allow %>
