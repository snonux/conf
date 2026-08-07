<% our $plugin_dir = '/usr/local/libexec/nagios'; -%>
{
  "EmailTo": "paul",
  "EmailFrom": "gogios@mx.buetow.org",
  "CheckTimeoutS": 10,
  "CheckConcurrency": 3,
  "MinNotifyIntervalS": 3600,
  "StateDir": "/var/run/gogios",
  "HTMLStatusFile": "/var/www/htdocs/buetow.org/self/gogios/index.html",
  <% if ($hostname eq 'fishfinger') { -%>
  "PeerURL": "https://blowfish.buetow.org/gogios/index.json",
  "PeerPrimaryName": "fishfinger.buetow.org",
  "PeerSecondaryName": "blowfish.buetow.org",
  <% } elsif ($hostname eq 'blowfish') { -%>
  "PeerURL": "https://fishfinger.buetow.org/gogios/index.json",
  "PeerPrimaryName": "fishfinger.buetow.org",
  "PeerSecondaryName": "blowfish.buetow.org",
  <% } -%>
  "PrometheusHosts": ["r0.wg0:30090", "r1.wg0:30090", "r2.wg0:30090"],
  "PrometheusOnlyIfNotExists": "/tmp/f3s_taken_down",
  "Checks": {
    <% for my $host (qw(master standby)) { -%>
    <%   for my $proto (4, 6) { -%>
    "Check Ping<%= $proto %> <%= $host %>.buetow.org": {
      "Plugin": "<%= $plugin_dir %>/check_ping",
      "Args": ["-H", "<%= $host %>.buetow.org", "-<%= $proto %>", "-w", "100,10%", "-c", "200,15%"],
      "RandomSpread": 10,
      "Retries": 3,
      "RetryInterval": 3
    },
    <%   } -%>
    <% } -%>
    <% for my $host (qw(blowfish fishfinger)) { -%>
    <%   for my $proto (4, 6) { -%>
    "Check Ping<%= $proto %> <%= $host %>.wg0.wan.buetow.org": {
      "Plugin": "<%= $plugin_dir %>/check_ping",
      "Args": ["-H", "<%= $wg0_ips->{$host}->{$proto} %>", "-<%= $proto %>", "-w", "100,20%", "-c", "200,30%"],
      "RandomSpread": 10,
      "Retries": 5,
      "RetryInterval": 3
    },
    <%   } -%>
    <% } -%>
    <% for my $host (qw(f0 f1 f2 r0 r1 r2 pi0 pi1)) { -%>
    <%   for my $proto (4, 6) { -%>
    "Check Ping<%= $proto %> <%= $host %>.wg0.wan.buetow.org": {
      "Plugin": "<%= $plugin_dir %>/check_ping",
      "Args": ["-H", "<%= $wg0_ips->{$host}->{$proto} %>", "-<%= $proto %>", "-w", "100,20%", "-c", "200,30%"],
      "OnlyIfNotExists": "/tmp/f3s_taken_down",
      "RandomSpread": 10,
      "Retries": 5,
      "RetryInterval": 3
    },
    <%   } -%>
    <% } -%>
    <% for my $host (qw(fishfinger blowfish)) { -%>
    "Check DTail <%= $host %>.buetow.org": {
      "Plugin": "/usr/local/bin/dtailhealth",
      "RunInterval": 3600,
      "RandomSpread": 10,
      "Args": ["--server", "<%= $host %>.buetow.org:2222"],
      "DependsOn": ["Check Ping4 <%= $host %>.buetow.org", "Check Ping6 <%= $host %>.buetow.org"]
    },
    <% } -%>
    <% for my $host (qw(fishfinger blowfish)) { -%>
    <%   for my $proto (4, 6) { -%>
    "Check Ping<%= $proto %> <%= $host %>.buetow.org": {
      "Plugin": "<%= $plugin_dir %>/check_ping",
      "RandomSpread": 10,
      "Args": ["-H", "<%= $host %>.buetow.org", "-<%= $proto %>", "-w", "100,10%", "-c", "200,15%"],
      "Retries": 3,
      "RetryInterval": 3
    },
    <%   } -%>
    "Check TLS Certificate <%= $host %>.buetow.org": {
      "Plugin": "<%= $plugin_dir %>/check_http",
      "RandomSpread": 10,
      "RunInterval": 3600,
      "Args": ["--sni", "-H", "<%= $host %>.buetow.org", "-C", "20" ],
      "DependsOn": ["Check Ping4 <%= $host %>.buetow.org", "Check Ping6 <%= $host %>.buetow.org"]
    },
    <% } -%>
    <% for my $host (qw(pi0 pi1)) { -%>
    "Check HTTP <%= $host %>.wg0.wan.buetow.org": {
      "Plugin": "<%= $plugin_dir %>/check_http",
      "OnlyIfNotExists": "/tmp/f3s_taken_down",
      "RandomSpread": 10,
      "Args": ["<%= $host %>.wg0.wan.buetow.org", "-4"]
    },
    <% } -%>
    <% for my $host (@$acme_hosts) {
         # Skip server hostnames - they have dedicated checks above without www/standby variants
         next if $host eq 'blowfish.buetow.org' or $host eq 'fishfinger.buetow.org';
         # Skip ipv4/ipv6 subdomains - they're included as SANs in parent cert and checked there
         next if $host =~ /^(ipv4|ipv6)\./;
         my $is_ipv6_only = $host =~ /^ipv6\./;
         my $is_ipv4_only = $host =~ /^ipv4\./;
    -%>
    <%   for my $prefix ('', 'standby.', 'www.') { -%>
    <%     my $depends_on = $prefix eq 'standby.' ? 'standby.buetow.org' : 'master.buetow.org'; -%>
    "Check TLS Certificate <%= $prefix . $host %>": {
      "Plugin": "<%= $plugin_dir %>/check_http",
      "RandomSpread": 10,
      "RunInterval": 3600,
      "Args": ["--sni", "-H", "<%= $prefix . $host %>", "-C", "20" ],
      "DependsOn": ["Check Ping4 <%= $depends_on %>", "Check Ping6 <%= $depends_on %>"]
    },
    <%     for my $proto (4, 6) { -%>
    "Check HTTP IPv<%= $proto %> <%= $prefix . $host %>": {
      "Plugin": "<%= $plugin_dir %>/check_http",
      "RandomSpread": 10,
      "Args": ["<%= $prefix . $host %>", "-<%= $proto %>"],
      "DependsOn": ["Check Ping<%= $proto %> <%= $depends_on %>"]
    },
    <%     } -%>
    <%   } -%>
    <% } -%>
    <%
    # HTTPS service checks.
    #
    # The port-80 checks above never reach the application: relayd/httpd answer
    # them with a 302 to HTTPS, which check_http reports as OK. They therefore
    # stayed green through a two-day audiobookshelf outage in August 2026.
    # These checks terminate TLS and hit the real backend.
    #
    # Bare hostnames only. Traefik has no ingress rules for the www./standby.
    # variants of the f3s services, so those answer 404 over HTTPS while
    # remaining valid over the port-80 redirect path.
    #
    # Services that legitimately answer with a non-2xx/3xx status at "/" carry
    # an explicit expected code, so the check tracks reachability rather than
    # the status code alone.
    #
    # NOTE: Rex renders a hash-prefixed template tag as a one-line Perl
    # comment, so multi-line commentary must live in a code block like this
    # one, and must never contain a closing template delimiter.
    -%>
    <% my %https_expect = (
         'player.f3s.buetow.org'   => 'HTTP/1.1 401',  # basic auth prompt
         'xplayer.f3s.buetow.org'  => 'HTTP/1.1 401',  # basic auth prompt
         'webdav.f3s.buetow.org'   => 'HTTP/1.1 401',  # basic auth prompt
         'garage.f3s.buetow.org'   => 'HTTP/1.1 403',  # S3 endpoint denies GET /
         'koreader.f3s.buetow.org' => 'HTTP/1.1 412',  # sync API, no root doc
         'pihole.f3s.buetow.org'   => 'HTTP/1.1 404',  # UI lives under /admin
         'anki.f3s.buetow.org'     => 'HTTP/1.1 404',  # sync API, no root doc
         'git.f3s.buetow.org'      => 'HTTP/1.1 404',
         'grafana.f3s.buetow.org'  => 'HTTP/1.1 404',
         'pkgrepo.f3s.buetow.org'  => 'HTTP/1.1 404',  # no autoindex at root
       );
       my %is_f3s = map { $_ => 1 } @$f3s_hosts;
       for my $host (@$acme_hosts) {
         # Server FQDNs have dedicated checks; ipv4./ipv6. have their own loop.
         next if $host eq 'blowfish.buetow.org' or $host eq 'fishfinger.buetow.org';
         next if $host =~ /^(ipv4|ipv6)\./;
         # ychat is currently unresponsive over HTTPS (socket timeout); adding a
         # check would alert on a known-broken legacy service. Re-enable by
         # removing this skip once it serves again.
         next if $host eq 'ychat.f3s.buetow.org';
         for my $proto (4, 6) {
    -%>
    "Check HTTPS IPv<%= $proto %> <%= $host %>": {
      "Plugin": "<%= $plugin_dir %>/check_http",
      "RandomSpread": 10,
      "RunInterval": 300,
      <%# f3s services are down by design while the cluster is taken down. -%>
      <% if ($is_f3s{$host}) { -%>
      "OnlyIfNotExists": "/tmp/f3s_taken_down",
      <% } -%>
      "Args": ["--sni", "-S", "-H", "<%= $host %>", "-<%= $proto %>"<% if ($https_expect{$host}) { %>, "-e", "<%= $https_expect{$host} %>"<% } %>],
      "DependsOn": ["Check Ping<%= $proto %> master.buetow.org"]
    },
    <%   } -%>
    <% } -%>
    <%# Special handling for ipv4/ipv6 subdomains - only check the appropriate IP version -%>
    <% for my $host (@$acme_hosts) {
         next unless $host =~ /^(ipv4|ipv6)\./;
         my $is_ipv6_only = $host =~ /^ipv6\./;
         my $is_ipv4_only = $host =~ /^ipv4\./;
         my $proto = $is_ipv6_only ? 6 : 4;
    -%>
    "Check HTTP IPv<%= $proto %> <%= $host %>": {
      "Plugin": "<%= $plugin_dir %>/check_http",
      "RandomSpread": 10,
      "Args": ["<%= $host %>", "-<%= $proto %>"],
      "DependsOn": ["Check Ping<%= $proto %> master.buetow.org"]
    },
    <% } -%>
    <% for my $host (qw(fishfinger blowfish)) { -%>
    <%   for my $proto (4, 6) { -%>
    "Check Dig <%= $host %>.buetow.org IPv<%= $proto %>": {
      "Plugin": "<%= $plugin_dir %>/check_dig",
      "RandomSpread": 10,
      "Args": ["-H", "<%= $host %>.buetow.org", "-l", "buetow.org", "-<%= $proto %>"],
      "DependsOn": ["Check Ping<%= $proto %> <%= $host %>.buetow.org"]
    },
    "Check SMTP <%= $host %>.buetow.org IPv<%= $proto %>": {
      "Plugin": "<%= $plugin_dir %>/check_smtp",
      "RandomSpread": 10,
      "Args": ["-H", "<%= $host %>.buetow.org", "-<%= $proto %>"],
      "DependsOn": ["Check Ping<%= $proto %> <%= $host %>.buetow.org"]
    },
    "Check Gemini TCP <%= $host %>.buetow.org IPv<%= $proto %>": {
      "Plugin": "<%= $plugin_dir %>/check_tcp",
      "RandomSpread": 10,
      "Args": ["-H", "<%= $host %>.buetow.org", "-p", "1965", "-<%= $proto %>"],
      "DependsOn": ["Check Ping<%= $proto %> <%= $host %>.buetow.org"]
    },
    <%   } -%>
    <% } -%>
    "Check Users <%= $hostname %>": {
      "Plugin": "<%= $plugin_dir %>/check_users",
      "RandomSpread": 10,
      "RunInterval": 600,
      "Args": ["-w", "2", "-c", "3"]
    },
    "Check SWAP <%= $hostname %>": {
      "Plugin": "<%= $plugin_dir %>/check_swap",
      "RandomSpread": 10,
      "RunInterval": 300,
<% if ($hostname eq 'fishfinger') { -%>
      "Args": ["-w", "20%", "-c", "10%"]
<% } else { -%>
      "Args": ["-w", "95%", "-c", "90%"]
<% } -%>
    },
    "Check Procs <%= $hostname %>": {
      "Plugin": "<%= $plugin_dir %>/check_procs",
      "RandomSpread": 10,
      "RunInterval": 300,
      "Args": ["-w", "100", "-c", "150"]
    },
    "Check Disk <%= $hostname %>": {
      "Plugin": "<%= $plugin_dir %>/check_disk",
      "RandomSpread": 10,
      "RunInterval": 300,
      "Args": ["-w", "30%", "-c", "10%"]
    },
    "Check Load <%= $hostname %>": {
      "Plugin": "<%= $plugin_dir %>/check_load",
      "RandomSpread": 10,
      "RunInterval": 300,
      "Args": ["-w", "2,1,1", "-c", "4,3,3"]
    },
    <%# Shuriken album freshness via status.json unix_epoch (written last on success, pushed by shuriken-sync); warn=3w, crit=5w; DependsOn the site HTTP checks. -%>
    "Check Shuriken Age irregular.ninja": {
      "Plugin": "/usr/local/bin/check_shuriken_age",
      "RandomSpread": 10,
      "RunInterval": 1800,
      "Args": ["-f", "/var/www/htdocs/irregular.ninja/status.json", "-w", "1814400", "-c", "3024000"],
      "DependsOn": ["Check HTTP IPv4 irregular.ninja", "Check HTTP IPv6 irregular.ninja"]
    },
    "Check Shuriken Age alt.irregular.ninja": {
      "Plugin": "/usr/local/bin/check_shuriken_age",
      "RandomSpread": 10,
      "RunInterval": 1800,
      "Args": ["-f", "/var/www/htdocs/alt.irregular.ninja/status.json", "-w", "1814400", "-c", "3024000"],
      "DependsOn": ["Check HTTP IPv4 alt.irregular.ninja", "Check HTTP IPv6 alt.irregular.ninja"]
    }
  }
}
