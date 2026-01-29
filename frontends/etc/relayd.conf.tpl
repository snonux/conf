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
    <% for my $host (@$acme_hosts) {
         # Skip server hostnames - each server only has its own cert, handled by dedicated keypair below
         next if $host eq 'blowfish.buetow.org' or $host eq 'fishfinger.buetow.org';
         # Skip ipv4/ipv6 subdomains - they use the parent cert as SANs
         next if $host =~ /^(ipv4|ipv6)\./;
    -%>
    tls keypair <%= $host %>
    <% unless (grep { $_ eq $host } @$f3s_hosts) { -%>
    tls keypair standby.<%= $host %>
    <% } -%>
    <% } -%>
    tls keypair <%= $hostname.'.'.$domain -%>

    # Enable WebSocket support
    http websockets

    match request header set "X-Forwarded-For" value "$REMOTE_ADDR"
    match request header set "X-Forwarded-Proto" value "https"

    # WebSocket headers - passed through for WebSocket connections
    pass header "Connection"
    pass header "Upgrade"
    pass header "Sec-WebSocket-Key"
    pass header "Sec-WebSocket-Version"
    pass header "Sec-WebSocket-Extensions"
    pass header "Sec-WebSocket-Protocol"

    # Explicitly route non-f3s hosts to localhost to prevent them from trying f3s backends
    <% for my $host (@$acme_hosts) {
         next if grep { $_ eq $host } @$f3s_hosts;
         for my $prefix (@prefixes) { -%>
    match request header "Host" value "<%= $prefix.$host -%>" forward to <localhost>
    <%   } } -%>

    # For f3s hosts: use relay-level failover (f3s -> localhost backup)
    # Registry is special: needs explicit routing to port 30001
    <% for my $host (@$f3s_hosts) { for my $prefix (@prefixes) {
          if ($host eq 'registry.f3s.buetow.org') { -%>
    match request header "Host" value "<%= $prefix.$host -%>" forward to <f3s_registry>
    <%   }
       } } -%>

    # Add cache-control headers to f3s fallback pages (served from localhost when cluster is down)
    match response header set "Cache-Control" value "no-cache, no-store, must-revalidate"
    match response header set "Pragma" value "no-cache"
    match response header set "Expires" value "0"
    }

relay "https4" {
    listen on <%= $vio0_ip %> port 443 tls
    protocol "https"
    # Primary: f3s cluster (with health checks) - Falls back to localhost when all hosts down
    forward to <f3s> port 80 check tcp
    forward to <localhost> port 8080
    # Registry uses separate port and table
    forward to <f3s_registry> port 30001 check tcp
}

relay "https6" {
    listen on <%= $ipv6address->($hostname) %> port 443 tls
    protocol "https"
    # Primary: f3s cluster (with health checks) - Falls back to localhost when all hosts down
    forward to <f3s> port 80 check tcp
    forward to <localhost> port 8080
    # Registry uses separate port and table
    forward to <f3s_registry> port 30001 check tcp
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
