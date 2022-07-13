<%
  our $primary = $is_primary->($vio0_ip);
  our $prefix = $primary ? '' : 'www.';
%>

# Plain HTTP for ACME and HTTPS redirect
<% for my $host (@$acme_hosts) { %>
server "<%= $prefix.$host %>" {
  listen on * port 80
  location "/.well-known/acme-challenge/*" {
    root "/acme"
    request strip 2
  }
  location * {
    block return 302 "https://$HTTP_HOST$REQUEST_URI"
  }
}
<% } %>

# Gemtexter hosts
<% for my $host (qw/foo.zone snonux.land/) { %>
server "<%= $prefix.$host %>" {
  listen on * tls port 443
  tls {
    certificate "/etc/ssl/<%= $prefix.$host %>.fullchain.pem"
    key "/etc/ssl/private/<%= $prefix.$host %>.key"
  }
  location * {
    root "/htdocs/gemtexter/<%= $host %>"
    directory auto index
  }
}
<% } %>

# DTail special host
server "<%= $prefix %>dtail.dev" {
  listen on * tls port 443
  tls {
    certificate "/etc/ssl/<%= $prefix %>dtail.dev.fullchain.pem"
    key "/etc/ssl/private/<%= $prefix %>dtail.dev.key"
  }
  location * {
    block return 302 "https://github.dtail.dev$REQUEST_URI"
  }
}

# Irregular Ninja special host
server "<%= $prefix %>irregular.ninja" {
  listen on * tls port 443
  tls {
    certificate "/etc/ssl/<%= $prefix %>irregular.ninja.fullchain.pem"
    key "/etc/ssl/private/<%= $prefix %>irregular.ninja.key"
  }
  location * {
    root "/htdocs/irregular.ninja"
    directory auto index
  }
}

# buetow.org special host.
server "<%= $prefix %>buetow.org" {
  listen on * tls port 443
  tls {
    certificate "/etc/ssl/<%= $prefix %>buetow.org.fullchain.pem"
    key "/etc/ssl/private/<%= $prefix %>buetow.org.key"
  }
  root "/htdocs/buetow.org"
  location match "/tmp/.*" {
    directory auto index
  }
  location match "/.*" {
    block return 302 "https://paul.buetow.org"
  }
}

<% if ($primary) { %>
server "paul.buetow.org" {
  listen on * tls port 443
  tls {
    certificate "/etc/ssl/paul.buetow.org.fullchain.pem"
    key "/etc/ssl/private/paul.buetow.org.key"
  }
  block return 302 "https://foo.zone/contact-information.html"
}
<% } %>

# Legacy hosts
server "snonux.de" {
  alias "www.snonux.de"
  listen on * port 80
  block return 302 "https://foo.zone$REQUEST_URI"
}

server "snonux.de" {
  alias "www.snonux.de"
  listen on * tls port 443
  tls {
    certificate "/etc/ssl/snonux.de.fullchain.pem"
    key "/etc/ssl/private/snonux.de.key"
  }
  block return 302 "https://foo.zone$REQUEST_URI"
}

server "foo.surf" {
  alias "www.foo.surf"
  listen on * port 80
  block return 302 "https://foo.zone$REQUEST_URI"
}

server "foo.surf" {
  alias "www.foo.surf"
  listen on * tls port 443
  tls {
    certificate "/etc/ssl/foo.surf.fullchain.pem"
    key "/etc/ssl/private/foo.surf.key"
  }
  block return 302 "https://foo.zone$REQUEST_URI"
}

server "sidewalk.ninja" {
  alias "www.sidewalk.ninja"
  listen on * port 80
  block return 302 "https://irregular.ninja$REQUEST_URI"
}

server "sidewalk.ninja" {
  alias "www.sidewalk.ninja"
  listen on * tls port 443
  tls {
    certificate "/etc/ssl/sidewalk.ninja.fullchain.pem"
    key "/etc/ssl/private/sidewalk.ninja.key"
  }
  block return 302 "https://irregular.ninja$REQUEST_URI"
}

# Defaults
server "default" {
  listen on * port 80
  block return 302 "https://foo.zone$REQUEST_URI"
}

server "default" {
  listen on * tls port 443
  tls {
    certificate "/etc/ssl/foo.zone.fullchain.pem"
    key "/etc/ssl/private/foo.zone.key"
  }
  block return 302 "https://foo.zone$REQUEST_URI"
}
