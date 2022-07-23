#!/bin/sh

<%
  our $primary = $is_primary->($vio0_ip);
  our $prefix = $primary ? '' : 'www.';
-%>

function handle_cert {
    host=$1
    # Create symlink, so that relayd also can read it.
    crt_path=/etc/ssl/$host
    if [ -e $crt_path.crt ]; then
        rm $crt_path.crt
    fi
    ln -s $crt_path.fullchain.pem $crt_path.crt
    # Requesting and renewing certificate.
    /usr/sbin/acme-client -v $host
}

has_update=no
<% for my $host (@$acme_hosts) { -%>
handle_cert <%= $prefix.$host %>
if [ $? -eq 0 ]; then
    has_update=yes
fi
<% } -%>

# Pick up the new certs.
if [ $has_update = yes ]; then
    /usr/sbin/rcctl reload httpd
    /usr/sbin/rcctl reload relayd
    /usr/sbin/rcctl restart smtpd
fi
