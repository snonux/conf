<%
  our @prefixes = ('', 'www.', 'standby.');
%>

# Plain HTTP for ACME and HTTPS redirect
<% for my $host (@$acme_hosts) { %>
<%   for my $prefix (@prefixes) { -%>
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
<%   } %>
<% } %>

# Current server's FQDN (e.g. for mail server ACME cert requests)
server "<%= "$hostname.$domain" %>" {
  listen on * port 80
  location "/.well-known/acme-challenge/*" {
    root "/acme"
    request strip 2
  }
  location * {
    block return 302 "https://<%= "$hostname.$domain" %>"
  }
}

server "<%= "$hostname.$domain" %>" {
  listen on * tls port 443
  tls {
    certificate "/etc/ssl/<%= "$hostname.$domain" %>.fullchain.pem"
    key "/etc/ssl/private/<%= "$hostname.$domain" %>.key"
  }
  location * {
    root "/htdocs/buetow.org/self"
    directory auto index
  }
}

# Gemtexter hosts
<% for my $host (qw/foo.zone paul.buetow.org snonux.foo/) { %>
<%   for my $prefix (@prefixes) { -%>
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
    <% if ($prefix eq 'www.') { -%>
    block return 302 "https://<%= $host %>$REQUEST_URI"
    <% } else { -%>
    root "/htdocs/gemtexter/<%= $host %>"
    directory auto index
    <% } -%>
  }
}
  <% } %>
<% } %>

# Redirect to paul.buetow.org
<% for my $prefix (@prefixes) { -%>
server "<%= $prefix %>buetow.org" {
  listen on * tls port 443
  tls {
    certificate "/etc/ssl/<%= $prefix %>buetow.org.fullchain.pem"
    key "/etc/ssl/private/<%= $prefix %>buetow.org.key"
  }
  location * {
    block return 302 "https://paul.buetow.org$REQUEST_URI"
  }
}
<% } -%>

# Redirect to gitub.dtail.dev
<% for my $prefix (@prefixes) { -%>
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
<% } -%>

# Irregular Ninja special hosts
<% for my $prefix (@prefixes) { -%>
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
<% } -%>

# Dory special host
<% for my $prefix (@prefixes) { -%>
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
<% } -%>

<% for my $prefix (@prefixes) { -%>
server "<%= $prefix %>solarcat.buetow.org" {
  listen on * tls port 443
  tls {
    certificate "/etc/ssl/<%= $prefix %>solarcat.buetow.org.fullchain.pem"
    key "/etc/ssl/private/<%= $prefix %>solarcat.buetow.org.key"
  }
  location * {
    root "/htdocs/joern/solarcat.buetow.org"
    directory auto index
  }
}
<% } -%>

<% for my $prefix (@prefixes) { -%>
server "<%= $prefix %>fotos.buetow.org" {
  listen on * tls port 443
  tls {
    certificate "/etc/ssl/<%= $prefix %>fotos.buetow.org.fullchain.pem"
    key "/etc/ssl/private/<%= $prefix %>fotos.buetow.org.key"
  }
  root "/htdocs/buetow.org/fotos"
  directory auto index
}
<% } -%>

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
