<% our @prefixes = ('', 'www.', 'standby.'); -%>
log connection

# Wireguard endpoints of the k3s cluster nodes running in FreeBSD bhyve Linux VMs via Wireguard tunnels
table <f3s> {
  192.168.2.120
  192.168.2.121
  192.168.2.122
}

# Same backends, separate table for registry service on port 30001
table <f3s_registry> {
  192.168.2.120
  192.168.2.121
  192.168.2.122
}

# Local OpenBSD httpd
table <localhost> {
  127.0.0.1
  ::1
}

http protocol "https" {
    <% for my $host (@$acme_hosts) { for my $prefix (@prefixes) { -%>
    tls keypair <%= $prefix.$host -%>
    <% } } -%>
    tls keypair <%= $hostname.'.'.$domain -%>

    match request header set "X-Forwarded-For" value "$REMOTE_ADDR"
    match request header set "X-Forwarded-Proto" value "https"
    
    # WebSocket support for audiobookshelf
    pass header "Connection"
    pass header "Upgrade"
    pass header "Sec-WebSocket-Key"
    pass header "Sec-WebSocket-Version"
    pass header "Sec-WebSocket-Extensions"
    pass header "Sec-WebSocket-Protocol"
    
    <% for my $host (@$f3s_hosts) { for my $prefix (@prefixes) { -%>
    <% if ($host eq 'registry.f3s.buetow.org') { -%>
    match request quick header "Host" value "<%= $prefix.$host -%>" forward to <f3s_registry>
    <% } else { -%>
    match request quick header "Host" value "<%= $prefix.$host -%>" forward to <f3s>
    <% } } } -%>
}

relay "https4" {
    listen on <%= $vio0_ip %> port 443 tls
    protocol "https"
    forward to <localhost> port 8080
    forward to <f3s_registry> port 30001 check tcp
    forward to <f3s> port 80 check tcp
}

relay "https6" {
    listen on <%= $ipv6address->($hostname) %> port 443 tls
    protocol "https"
    forward to <localhost> port 8080
    forward to <f3s_registry> port 30001 check tcp
    forward to <f3s> port 80 check tcp
}

tcp protocol "gemini" {
    tls keypair foo.zone
    tls keypair stats.foo.zone
    tls keypair snonux.foo
    tls keypair paul.buetow.org
    tls keypair standby.foo.zone
    tls keypair standby.stats.foo.zone
    tls keypair standby.snonux.foo
    tls keypair standby.paul.buetow.org
}

relay "gemini4" {
    listen on <%= $vio0_ip %> port 1965 tls
    protocol "gemini"
    forward to 127.0.0.1 port 11965
}

relay "gemini6" {
    listen on <%= $ipv6address->($hostname) %> port 1965 tls
    protocol "gemini"
    forward to 127.0.0.1 port 11965
}
