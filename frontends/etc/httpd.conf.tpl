<% our @prefixes = ('', 'www.', 'standby.'); -%>
# Plain HTTP for ACME and HTTPS redirect
<% for my $host (@$acme_hosts) { for my $prefix (@prefixes) { -%>
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
<% } } -%>

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
  listen on * port 8080
  log style forwarded 
  location * {
    root "/htdocs/buetow.org/self"
    directory auto index
  }
}

# Gemtexter hosts
<% for my $host (qw/foo.zone/) { for my $prefix (@prefixes) { -%>
server "<%= $prefix.$host %>" {
  listen on * port 8080
  log style forwarded 
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
<% } } -%>

# Redirect to paul.buetow.org
<% for my $prefix (@prefixes) { -%>
server "<%= $prefix %>buetow.org" {
  listen on * port 8080
  log style forwarded 
  location * {
    block return 302 "https://paul.buetow.org$REQUEST_URI"
  }
}

server "<%= $prefix %>snonux.foo" {
  listen on * port 8080
  log style forwarded 
  location * {
    block return 302 "https://foo.zone$REQUEST_URI"
  }
}

server "<%= $prefix %>paul.buetow.org" {
  listen on * port 8080
  log style forwarded 
  location * {
    block return 302 "https://foo.zone/about$REQUEST_URI"
  }
}
<% } -%>

# Redirect to gitub.dtail.dev
<% for my $prefix (@prefixes) { -%>
server "<%= $prefix %>dtail.dev" {
  listen on * port 8080
  log style forwarded 
  location * {
    block return 302 "https://github.dtail.dev$REQUEST_URI"
  }
}
<% } -%>

# Irregular Ninja special hosts
<% for my $prefix (@prefixes) { -%>
server "<%= $prefix %>irregular.ninja" {
  listen on * port 8080
  log style forwarded 
  location * {
    root "/htdocs/irregular.ninja"
    directory auto index
  }
}
<% } -%>

<% for my $prefix (@prefixes) { -%>
server "<%= $prefix %>alt.irregular.ninja" {
  listen on * port 8080
  log style forwarded 
  location * {
    root "/htdocs/alt.irregular.ninja"
    directory auto index
  }
}
<% } -%>

# Dory special host
<% for my $prefix (@prefixes) { -%>
server "<%= $prefix %>dory.buetow.org" {
  listen on * port 8080
  log style forwarded 
  location * {
    root "/htdocs/joern/dory.buetow.org"
    directory auto index
  }
}
<% } -%>

<% for my $prefix (@prefixes) { -%>
server "<%= $prefix %>solarcat.buetow.org" {
  listen on * port 8080
  log style forwarded 
  location * {
    root "/htdocs/joern/solarcat.buetow.org"
    directory auto index
  }
}
<% } -%>

<% for my $prefix (@prefixes) { -%>
server "<%= $prefix %>fotos.buetow.org" {
  listen on * port 8080
  log style forwarded 
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
  listen on * port 8080
  log style forwarded 
  block return 302 "https://foo.zone$REQUEST_URI"
}
