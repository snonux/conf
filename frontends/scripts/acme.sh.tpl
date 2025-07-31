#!/bin/sh

MY_IP=`ifconfig vio0 | awk '$1 == "inet" { print $2 }'`

# New hosts may not have a cert, just copy foo.zone as a
# placeholder, so that services can at least start proprely.
# cert will be updated with next acme-client runs!
ensure_placeholder_cert () {
    host=$1
    copy_from=foo.zone

    if [ ! -f /etc/ssl/$host.crt ]; then
        cp -v /etc/ssl/$copy_from.crt /etc/ssl/$host.crt
        cp -v /etc/ssl/$copy_from.fullchain.pem /etc/ssl/$host.fullchain.pem
        cp -v /etc/ssl/private/$copy_from.key /etc/ssl/private/$host.key
    fi
}

handle_cert () {
    host=$1
    host_ip=`host $host | awk '/has address/ { print $(NF) }'`

    grep -q "^server \"$host\"" /etc/httpd.conf
    if [ $? -ne 0 ]; then
        echo "Host $host not configured in httpd, skipping..."
        return
    fi
    ensure_placeholder_cert "$host"

    if [ "$MY_IP" != "$host_ip" ]; then
        echo "Not serving $host, skipping..."
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
    # TLS offloading fully moved to relayd now
    # /usr/sbin/rcctl reload httpd

    /usr/sbin/rcctl reload relayd
    /usr/sbin/rcctl restart smtpd
fi
