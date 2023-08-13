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

# Current server's FQDN (e.g. for mail server ACME cert requests)
server "<%= "$hostname.$domain" %>" {
  listen on * port 80
  location "/.well-known/acme-challenge/*" {
    root "/acme"
    request strip 2
  }
  location * {
    block return 302 "https://<%= $prefix %>buetow.org"
  }
}

# Gemtexter hosts
<% for my $host (qw/foo.zone snonux.land paul.buetow.org/) { %>
server "<%= $prefix.$host %>" {
  listen on * tls port 443
  tls {
    certificate "/etc/ssl/<%= $prefix.$host %>.fullchain.pem"
    key "/etc/ssl/private/<%= $prefix.$host %>.key"
  }
  location "/.git*" {
    block return 302 "https://<%= $prefix.$host %>"
  }
  location * {
    root "/htdocs/gemtexter/<%= $host %>"
    directory auto index
  }
}
<% } %>

# Redirect to paul.buetow.org
<% for my $host (qw/buetow.org paul.cyou snonux.foo/) { %>
server "<%= $prefix.$host %>" {
  listen on * tls port 443
  tls {
    certificate "/etc/ssl/<%= $prefix.$host %>.fullchain.pem"
    key "/etc/ssl/private/<%= $prefix.$host %>.key"
  }
  location * {
    block return 302 "https://<%= $prefix %>paul.buetow.org"
  }
}
<% } %>

# Redirec to to gitub.dtail.dev
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

# Dory special host
server "<%= $prefix %>dory.buetow.org" {
  listen on * tls port 443
  tls {
    certificate "/etc/ssl/<%= $prefix %>dory.buetow.org.fullchain.pem"
    key "/etc/ssl/private/<%= $prefix %>dory.buetow.org.key"
  }
  location * {
    root "/htdocs/joern/dory.buetow.org"
    directory auto index
  }
}

server "<%= $prefix %>tmp.buetow.org" {
  listen on * tls port 443
  tls {
    certificate "/etc/ssl/<%= $prefix %>tmp.buetow.org.fullchain.pem"
    key "/etc/ssl/private/<%= $prefix %>tmp.buetow.org.key"
  }
  root "/htdocs/buetow.org/tmp"
  directory auto index
}

server "<%= $prefix %>tmp.foo.zone" {
  listen on * tls port 443
  tls {
    certificate "/etc/ssl/<%= $prefix %>tmp.foo.zone.fullchain.pem"
    key "/etc/ssl/private/<%= $prefix %>tmp.foo.zone.key"
  }
  root "/htdocs/buetow.org/tmp"
  directory auto index
}

server "<%= $prefix %>footos.buetow.org" {
  listen on * tls port 443
  tls {
    certificate "/etc/ssl/<%= $prefix %>footos.buetow.org.fullchain.pem"
    key "/etc/ssl/private/<%= $prefix %>footos.buetow.org.key"
  }
  root "/htdocs/buetow.org/footos"
  directory auto index
}

server "<%= $prefix %>fotos.buetow.org" {
  listen on * tls port 443
  tls {
    certificate "/etc/ssl/<%= $prefix %>fotos.buetow.org.fullchain.pem"
    key "/etc/ssl/private/<%= $prefix %>fotos.buetow.org.key"
  }
  root "/htdocs/buetow.org/fotos"
  directory auto index
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
