<% our @prefixes = ('', 'www.', 'standby.'); -%>
log connection

# Wireguard endpoints of the k3s cluster nodes running in FreeBSD bhyve Linux VMs via Wireguard tunnels
table <f3s> {
  192.168.2.120
  192.168.2.121
  192.168.2.122
}

# Static f3s landing page served from the Raspberry Pi HTTP nodes over WireGuard
table <f3s_static> {
  192.168.2.203
  192.168.2.204
}

# Local relayd hop for the static landing page, used to get a real localhost fallback
table <f3s_static_proxy> {
  127.0.0.1
  ::1
}

# Same backends, separate table for registry service on port 30001
table <f3s_registry> {
  192.168.2.120
  192.168.2.121
  192.168.2.122
}

# Jellyfin NodePorts (bypasses Traefik to avoid double proxy issues)
table <f3s_jellyfin> {
  192.168.2.120
  192.168.2.121
  192.168.2.122
}

# Anki sync server NodePort (bypasses Traefik to fix zstd stream failures)
table <f3s_anki> {
  192.168.2.120
  192.168.2.121
  192.168.2.122
}

# Garage S3 API on f0/f1/f2 FreeBSD hosts (WireGuard)
table <garage> {
  192.168.2.130
  192.168.2.131
  192.168.2.132
}

# Forgejo git+ssh backends (NodePort 30222 on the k3s nodes, over WireGuard).
# Separate table from <f3s> so the health check tracks the SSH port specifically:
# the web UI can be up while the built-in SSH server is not.
table <forgejo_ssh> {
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
     tls keypair standby.<%= $host %>
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

    # code.f3s.buetow.org (Forgejo web UI) drew a crawler that hammered the
    # expensive blame/commit/diff pages hard enough to saturate the home
    # fiber uplink. Block it on the standard port here; the same host stays
    # reachable at https://code.f3s.buetow.org:2443/ (see the "forgejo-alt"
    # relay below) for anyone who already knows the non-standard port, and
    # git+ssh on 2022 is untouched since that relay is a plain TCP forward
    # with no Host-header awareness. Port 80 stays alive too -- it never
    # reaches Forgejo (relayd doesn't proxy :80 to the cluster, httpd just
    # does ACME-challenge/redirect there) and killing it would break the
    # cert renewal that the :2443 listener depends on.
    <% for my $prefix (@prefixes) { -%>
    block request header "Host" value "<%= $prefix %>code.f3s.buetow.org"
    <% } -%>

    # c-git.f3s.buetow.org (legacy cgit git-server) used to get the same
    # crawler-mitigation block here. The workload was retired 2026-08-30 after
    # its 14-day post-Forgejo-cutover retention window expired (task 3x0): the
    # ArgoCD Application, helm chart and NodePort 30022 ssh path are gone, and
    # the hostname was dropped from @f3s_hosts in Rexfile, so it no longer
    # gets a TLS keypair or a routing rule at all -- there is nothing left to
    # explicitly block. This comment is left as a pointer for anyone who finds
    # old bookmarks/monitoring still referencing the hostname.

    # Explicitly route non-f3s hosts to localhost to prevent them from trying f3s backends
    <% for my $host (@$acme_hosts) {
         next if grep { $_ eq $host } @$f3s_hosts;
         next if $host eq 'snonux.foo';
         for my $prefix (@prefixes) { -%>
    match request header "Host" value "<%= $prefix.$host -%>" forward to <localhost>
    <%   } } -%>

    # For f3s hosts: use relay-level failover (f3s -> localhost backup)
    # Registry is special: needs explicit routing to port 30001
    <% for my $host (@$f3s_hosts) {
          for my $prefix (@prefixes) {
              if ($host eq 'f3s.buetow.org') {
    -%>
    match request header "Host" value "<%= $prefix.$host -%>" forward to <f3s_static_proxy>
    <%         } elsif ($host eq 'registry.f3s.buetow.org') {
    -%>
    match request header "Host" value "<%= $prefix.$host -%>" forward to <f3s_registry>
    <%         } elsif ($host eq 'jellyfin.f3s.buetow.org') {
    -%>
    match request header "Host" value "<%= $prefix.$host -%>" forward to <f3s_jellyfin>
    <%         } elsif ($host eq 'anki.f3s.buetow.org') {
    -%>
    match request header "Host" value "<%= $prefix.$host -%>" forward to <f3s_anki>
    <%         # garage.f3s.buetow.org is the S3 endpoint itself, and
               # <bucket>.garage.f3s.buetow.org is how a client that cannot
               # force path-style addressing asks for a single bucket -- Garage
               # maps that name back to the bucket via its root_domain. Both
               # go to the same backend. Bucket hosts come from @garage_hosts,
               # which is pushed onto @f3s_hosts, so they are matched here by
               # suffix rather than being listed one by one.
               } elsif ($host eq 'garage.f3s.buetow.org'
                     or $host =~ /\.garage\.f3s\.buetow\.org$/) {
    -%>
    match request header "Host" value "<%= $prefix.$host -%>" forward to <garage>
    <%         }
          }
    } -%>

    # www.snonux.foo: redirect to snonux.foo via localhost httpd
    match request header "Host" value "www.snonux.foo" forward to <localhost>
    # snonux.foo: same relay hop as f3s.buetow.org (Pis then localhost f3s_fallback). relayd cannot rewrite
    # URL paths; use https://snonux.foo/snonux/... or map Host on the static servers so / serves that tree.
    <% for my $host (qw/snonux.foo/) {
         for my $prefix ('', 'standby.') { -%>
    match request header "Host" value "<%= $prefix.$host -%>" forward to <f3s_static_proxy>
    <%   } } -%>

    # Keep the f3s fallback pages (served from localhost httpd when the cluster
    # is down) out of browser caches, so nobody keeps seeing "cluster is down"
    # after the cluster recovers.
    #
    # These rules used to be unscoped, so they rewrote EVERY response through
    # this relay -- including everything the k3s cluster serves. That silently
    # defeated upstream cache headers cluster-wide: cgit sets "expires 30d" on
    # its CSS and logo, but browsers re-fetched them on every page view.
    #
    # relayd cannot filter a response by the backend table that produced it, so
    # scope on the Server header instead: the local httpd is the only backend
    # that answers "OpenBSD httpd" (the cluster answers nginx, the Pis
    # bozohttpd). A "header set" cannot be combined with a header match in one
    # rule, hence the tag: it is sticky for the connection, and a later
    # response rule picks it up with "tagged".
    match response header "Server" value "OpenBSD httpd" tag "HTTPD_FALLBACK"
    match response tagged "HTTPD_FALLBACK" header set "Cache-Control" value "no-cache, no-store, must-revalidate"
    match response tagged "HTTPD_FALLBACK" header set "Pragma" value "no-cache"
    match response tagged "HTTPD_FALLBACK" header set "Expires" value "0"
    }

relay "https4" {
    listen on <%= $ipv4address->($hostname) %> port 443 tls
    protocol "https"
    session timeout 300
    # Primary: f3s cluster (with health checks) - Falls back to localhost when all hosts down
    forward to <f3s> port 80 check tcp
    forward to <localhost> port 8080 check http "/" code 200
    # Static landing page is routed through a local relay so it can fall back to localhost.
    # Listed after localhost so it does NOT become a general fallback for k3s failures;
    # only reached via explicit "match ... forward to <f3s_static_proxy>" rules.
    forward to <f3s_static_proxy> port 18080 check tcp
    # Registry uses separate port and table
    forward to <f3s_registry> port 30001 check tcp
    # Jellyfin uses NodePorts (bypasses Traefik)
    forward to <f3s_jellyfin> port 30096 check tcp
    # Anki sync server NodePort (bypasses Traefik to fix zstd stream failures)
    forward to <f3s_anki> port 30800 check tcp
    # Garage S3 API on FreeBSD hosts
    forward to <garage> port 3900 check tcp
}

relay "https6" {
    listen on <%= $ipv6address->($hostname) %> port 443 tls
    protocol "https"
    session timeout 300
    # Primary: f3s cluster (with health checks) - Falls back to localhost when all hosts down
    forward to <f3s> port 80 check tcp
    forward to <localhost> port 8080 check http "/" code 200
    # Static landing page is routed through a local relay so it can fall back to localhost.
    # Listed after localhost so it does NOT become a general fallback for k3s failures;
    # only reached via explicit "match ... forward to <f3s_static_proxy>" rules.
    forward to <f3s_static_proxy> port 18080 check tcp
    # Registry uses separate port and table
    forward to <f3s_registry> port 30001 check tcp
    # Jellyfin uses NodePorts (bypasses Traefik)
    forward to <f3s_jellyfin> port 30096 check tcp
    # Anki sync server NodePort (bypasses Traefik to fix zstd stream failures)
    forward to <f3s_anki> port 30800 check tcp
    # Garage S3 API on FreeBSD hosts
    forward to <garage> port 3900 check tcp
}

# Jellyfin alternative ports for Android app discovery
# Use the same "https" protocol to get X-Forwarded headers
# relay "jellyfin-8096-ipv4" {
#     listen on <%= $ipv4address->($hostname) %> port 8096 tls
#     protocol "https"
#     forward to <f3s_jellyfin> port 30096 check tcp
# }

# relay "jellyfin-8096-ipv6" {
#     listen on <%= $ipv6address->($hostname) %> port 8096 tls
#     protocol "https"
#     forward to <f3s_jellyfin> port 30096 check tcp
# }

# relay "jellyfin-8920-ipv4" {
#     listen on <%= $ipv4address->($hostname) %> port 8920 tls
#     protocol "https"
#     forward to <f3s_jellyfin> port 30096 check tcp
# }

# relay "jellyfin-8920-ipv6" {
#     listen on <%= $ipv6address->($hostname) %> port 8920 tls
#     protocol "https"
#     forward to <f3s_jellyfin> port 30096 check tcp
# }

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
    listen on <%= $ipv4address->($hostname) %> port 1965 tls
    protocol "gemini"
    forward to 127.0.0.1 port 11965
}

relay "gemini6" {
    listen on <%= $ipv6address->($hostname) %> port 1965 tls
    protocol "gemini"
    forward to 127.0.0.1 port 11965
}

# Forgejo web UI, alternate port. Standard :443 blocks code.f3s.buetow.org
# (see the "block quick" rule in the "https" protocol above, added after a
# crawler hammering blame/commit/diff pages saturated the home fiber link).
# This keeps the UI reachable for anyone who knows the non-standard port,
# same idea as git+ssh living on 2022 instead of 22.
http protocol "forgejo-alt" {
    tls keypair code.f3s.buetow.org

    http websockets

    match request header set "X-Forwarded-For" value "$REMOTE_ADDR"
    match request header set "X-Forwarded-Proto" value "https"

    pass header "Connection"
    pass header "Upgrade"
    pass header "Sec-WebSocket-Key"
    pass header "Sec-WebSocket-Version"
    pass header "Sec-WebSocket-Extensions"
    pass header "Sec-WebSocket-Protocol"

    match request header "Host" value "code.f3s.buetow.org" forward to <f3s>
}

relay "forgejo_alt4" {
    listen on <%= $ipv4address->($hostname) %> port 2443 tls
    protocol "forgejo-alt"
    forward to <f3s> port 80 check tcp
}

relay "forgejo_alt6" {
    listen on <%= $ipv6address->($hostname) %> port 2443 tls
    protocol "forgejo-alt"
    forward to <f3s> port 80 check tcp
}

# Forgejo git+ssh.
#
# Port 2022, deliberately not 22: leaving the forge off the default port keeps
# it out of the way of the mass scanners that hammer 22 continuously. That is
# noise reduction, not security -- the actual protection is that Forgejo's SSH
# server does key-only auth for git operations and offers no shell.
#
# 2222 was the obvious alternative but is already taken here by dserver (DTail).
#
# Plain TCP relay: no "protocol" line, so relayd forwards the stream untouched.
# TLS is not involved and must not be -- SSH does its own transport security,
# and the client verifies Forgejo's own host key at the far end.
#
# Only the gateway currently holding the code.f3s.buetow.org address actually
# receives connections; the other listens harmlessly.
relay "forgejo_ssh4" {
    listen on <%= $ipv4address->($hostname) %> port 2022
    forward to <forgejo_ssh> port 30222 check tcp
}

relay "forgejo_ssh6" {
    listen on <%= $ipv6address->($hostname) %> port 2022
    forward to <forgejo_ssh> port 30222 check tcp
}

relay "f3s_static_proxy4" {
    listen on 127.0.0.1 port 18080
    forward to <f3s_static> port 80 check tcp
    forward to <localhost> port 8080 check http "/" code 200
}

relay "f3s_static_proxy6" {
    listen on ::1 port 18080
    forward to <f3s_static> port 80 check tcp
    forward to <localhost> port 8080 check http "/" code 200
}
