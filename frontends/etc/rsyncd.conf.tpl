<% my $allow = '108.160.134.135,2401:c080:1000:45af:5400:3ff:fec6:ca1d,*.buetow.org,localhost'; %>
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
