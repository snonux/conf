<% our @prefixes = ('', 'www.', 'standby.'); -%>
# Plain HTTP for ACME and HTTPS redirect
<% for my $host (@$acme_hosts) {
     # Skip current server's hostname - handled by dedicated block below
     next if $host eq "$hostname.$domain";
     for my $prefix (@prefixes) { -%>
server "<%= $prefix.$host %>" {
  listen on * port 80
  log style forwarded
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
  log style forwarded 
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
<% for my $host (qw/foo.zone stats.foo.zone/) { for my $prefix (@prefixes) { -%>
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

# Redirect blog to foo.zone
server "<%= $prefix %>blog.buetow.org" {
  listen on * port 8080
  log style forwarded 
  location * {
    block return 302 "https://foo.zone$REQUEST_URI"
  }
}

server "<%= $prefix %>snonux.foo" {
  listen on * port 8080
  log style forwarded 
  location * {
    block return 302 "https://foo.zone/about$REQUEST_URI"
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

<% for my $prefix (@prefixes) { -%>
server "<%= $prefix %>dtail.dev" {
  listen on * port 8080
  log style forwarded 
  location * {
    block return 302 "https://codeberg.org/snonux/dtail"
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

# joern special host
<% for my $prefix (@prefixes) { -%>
server "<%= $prefix %>joern.buetow.org" {
  listen on * port 8080
  log style forwarded 
  location * {
    root "/htdocs/joern/"
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

# ecat special host
<% for my $prefix (@prefixes) { -%>
server "<%= $prefix %>ecat.buetow.org" {
  listen on * port 8080
  log style forwarded 
  location * {
    root "/htdocs/joern/ecat.buetow.org"
    directory auto index
  }
}
<% } -%>

# gogios special host
<% for my $prefix (@prefixes) { -%>
server "<%= $prefix %>gogios.buetow.org" {
  listen on * port 8080
  log style forwarded
  location * {
    root "/htdocs/buetow.org/self/gogios"
    directory auto index
  }
}
<% } -%>

# Fallback for f3s hosts - serve fallback page for ALL paths
<% for my $host (@$f3s_hosts) { for my $prefix (@prefixes) { -%>
server "<%= $prefix.$host %>" {
  listen on * port 8080
  log style forwarded
  location * {
    # Rewrite all requests to /index.html to show fallback page regardless of path
    request rewrite "/index.html"
    root "/htdocs/f3s_fallback"
  }
}
<% } } -%>

# Defaults
server "default" {
  listen on * port 80
  log style forwarded 
  block return 302 "https://foo.zone$REQUEST_URI"
}

server "default" {
  listen on * port 8080
  log style forwarded 
  block return 302 "https://foo.zone$REQUEST_URI"
}
