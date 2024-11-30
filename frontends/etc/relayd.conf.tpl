log connection

<%
  our @prefixes = ('', 'www.', 'standby.');
%>

tcp protocol "https" {
<% for my $host (@$acme_hosts) { -%>
<%   for my $prefix (@prefixes) { -%>
    tls keypair <%= $prefix.$host -%>
<%   } -%>
<% } -%>
    tls keypair <%= $hostname.'.'.$domain -%>
}

relay "https4" {
    listen on <%= $vio0_ip %> port 443 tls
    protocol "https"
    forward to 127.0.0.1 port 8080
}

relay "https6" {
    listen on <%= $ipv6address->($hostname) %> port 443 tls
    protocol "https"
    forward to ::1 port 8080
}

tcp protocol "gemini" {
    tls keypair foo.zone
    tls keypair snonux.foo
    tls keypair paul.buetow.org
    tls keypair standby.foo.zone
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
