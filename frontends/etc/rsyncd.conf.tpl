<% my $allow = '*.wg0.wan.buetow.org,*.wg0,localhost'; %>
max connections = 5
timeout = 300

[joernshtdocs]
comment = Joerns htdocs
path = /var/www/htdocs/joern
read only = yes
list = yes
uid = www
gid = www
hosts allow = <%= $allow %>

# [publicgemini]
# comment = Public Gemini capsule content
# path = /var/gemini
# read only = yes
# list = yes
# uid = www
# gid = www
# hosts allow = <%= $allow %>

# [sslcerts]
# comment = TLS certificates
# path = /etc/ssl
# read only = yes
# list = yes
# hosts allow = <%= $allow %>
