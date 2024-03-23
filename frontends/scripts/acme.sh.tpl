#!/bin/sh

MY_IP=`ifconfig vio0 | awk '$1 == "inet" { print $2 }'`

function handle_cert {
    host=$1
    host_ip=`host $host | awk '/has address/ { print $(NF) }'`
    if [ "$MY_IP" != "$host_ip" ]; then
        echo "Not serving $host, skipping..."
        return
    fi
    grep -q "^server \"$host\"" /etc/httpd.conf
    if [ $? -ne 0 ]; then
        echo "Host $host not configured in httpd, skipping..."
        return
    fi

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
<%   for my $prefix ('', 'www.', 'standby.') { -%>
handle_cert <%= $prefix.$host %>
if [ $? -eq 0 ]; then
    has_update=yes
fi
<%   } -%>
<% } -%>

# Current server's FQDN (e.g. for mail server certs)
handle_cert <%= "$hostname.$domain" %>
if [ $? -eq 0 ]; then
    has_update=yes
fi

# Pick up the new certs.
if [ $has_update = yes ]; then
    /usr/sbin/rcctl reload httpd
    /usr/sbin/rcctl reload relayd
    /usr/sbin/rcctl restart smtpd
fi
