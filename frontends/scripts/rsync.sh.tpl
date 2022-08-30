#!/bin/sh

PATH=$PATH:/usr/local/bin

<% unless ($is_primary->($vio0_ip)) { %>
/usr/local/bin/rsync -av --delete rsync://blowfish.buetow.org/publicgemini/ /var/gemini
/usr/local/bin/rsync -av --delete rsync://blowfish.buetow.org/publichttp/ /var/www/htdocs
<% } %>
