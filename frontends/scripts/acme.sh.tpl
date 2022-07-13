#!/bin/sh

<%
  our $primary = $is_primary->($vio0_ip);
  our $prefix = $primary ? '' : 'www.';
-%>

<% for my $host (@$acme_hosts) { -%>
# Requesting and renewing certificate.
/usr/sbin/acme-client -v <%= $prefix.$host %>
# Create symlink, so that relayd also can read it.
crt_path=/etc/ssl/<%= $prefix.$host %>
if [ -e $crt_path.crt ]; then
    rm $crt_path.crt
fi
ln -s $crt_path.fullchain.pem $crt_path.crt

<% } -%>

# Pick up the new certs.
/usr/sbin/rcctl reload httpd
/usr/sbin/rcctl reload relayd
