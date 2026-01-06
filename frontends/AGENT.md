# Frontend Infrastructure Knowledge

## Architecture Overview

### Request Flow
```
Internet → relayd (port 443) → routing decision → httpd (port 8080) or f3s cluster (port 80)
```

### Key Components

**relayd** - Reverse proxy that:
- Terminates TLS on port 443 (IPv4 and IPv6)
- Routes requests based on Host header matching
- Has two backend pools:
  - `<localhost>` (127.0.0.1, ::1) - Routes to local httpd on port 8080
  - `<f3s>` (192.168.2.120-122) - Routes to f3s k3s cluster on port 80
- Falls back to f3s cluster when no explicit routing match exists

**httpd** - OpenBSD httpd that:
- Listens on port 8080 (behind relayd)
- Listens on port 80 for ACME challenges and HTTP→HTTPS redirects
- Serves static content for various domains
- Has server-specific blocks for each server's own hostname

**Rexfile** - Configuration management using Rex (Perl):
- Defines configuration arrays (`@acme_hosts`, `@f3s_hosts`, etc.)
- Templates use these arrays to generate httpd and relayd configs
- Deploys to both blowfish and fishfinger servers in parallel
- Each server receives templates processed with its own `$hostname` value

## Configuration Arrays

### @acme_hosts
Controls which hosts get:
- ACME certificate requests
- HTTP port 80 server blocks for ACME challenges
- Explicit routing rules in relayd to `<localhost>`

**Critical**: Hosts NOT in `@acme_hosts` will fall through to f3s cluster backends in relayd.

### @f3s_hosts
Hosts served by the f3s k3s cluster:
- Get fallback page served by httpd
- Special routing rules in relayd to f3s backends

### @prefixes
Array: `('', 'www.', 'standby.')`

Used in loops to create hostname variants:
- `foo.zone`
- `www.foo.zone`
- `standby.foo.zone`

## Template Processing

Rex processes `.tpl` files using embedded Perl:

```perl
<% ... -%>        # Perl code (- suppresses trailing newline)
<%= $var %>       # Print variable value
```

Templates are processed **per-server** with different values:
- `$hostname` = "blowfish" or "fishfinger"
- `$domain` = "buetow.org"
- `$hostname.$domain` = "blowfish.buetow.org" or "fishfinger.buetow.org"

## Routing Configuration

### Explicit Routing Rules (relayd.conf.tpl:45-50)

```perl
<% for my $host (@$acme_hosts) {
     next if grep { $_ eq $host } @$f3s_hosts;
     for my $prefix (@prefixes) { -%>
match request header "Host" value "<%= $prefix.$host -%>" forward to <localhost>
```

- Only hosts in `@acme_hosts` get explicit routing to `<localhost>`
- Excludes f3s hosts (they have separate routing)
- Creates rules for all prefixes ('', 'www.', 'standby.')

### Routing Logic

**Routing is explicit, not implicit**: Just because httpd has a server block doesn't mean relayd will route to it. The routing decision happens in relayd based on:

1. Explicit Host header match → route to specified backend
2. No match → fall through to default relay backends (f3s cluster first, then localhost)

## TLS Certificate Management

### Certificate Loading (relayd.conf.tpl:24-31)

```perl
http protocol "https" {
    <% for my $host (@$acme_hosts) { -%>
    tls keypair <%= $host %>
    tls keypair standby.<%= $host %>
    <% } -%>
    tls keypair <%= $hostname.'.'.$domain -%>
```

**Critical insight**: In multi-server deployments, each server only has its own TLS certificate.

- blowfish has: `blowfish.buetow.org.crt` (NOT fishfinger's cert)
- fishfinger has: `fishfinger.buetow.org.crt` (NOT blowfish's cert)

When the template runs on blowfish, it tries to load certs for ALL hosts in `@acme_hosts`. If fishfinger.buetow.org is in the array, relayd will fail to start because that cert doesn't exist on blowfish.

**Solution pattern**: Skip server-specific hostnames in the loop, use dedicated keypair line:
```perl
<% for my $host (@$acme_hosts) {
     next if $host eq 'blowfish.buetow.org' or $host eq 'fishfinger.buetow.org'; -%>
```

The line `tls keypair <%= $hostname.'.'.$domain -%>` loads the correct cert for each server.

## Server Block Management

### httpd.conf.tpl Patterns

**ACME and redirect blocks (port 80)**:
```perl
<% for my $host (@$acme_hosts) {
     next if $host eq "$hostname.$domain";  # Skip current server
     for my $prefix (@prefixes) { -%>
server "<%= $prefix.$host %>" {
  listen on * port 80
```

**Why skip current server**: Each server has a dedicated "Current server's FQDN" block:

```perl
server "<%= "$hostname.$domain" %>" {
  listen on * port 80
  ...
}
```

Without the skip, adding server hostnames to `@acme_hosts` creates duplicate server blocks, causing httpd to fail with "server defined twice" error.

### Content Serving Blocks (port 8080)

Different patterns based on content type:
- **Gemtexter sites**: Serve from `/htdocs/gemtexter/<host>`
- **Server self**: Serve from `/htdocs/buetow.org/self`
- **Special hosts**: Custom root paths (e.g., gogios, joern, dory)
- **f3s fallback**: Rewrite all to `/index.html` for cluster-down message

## Server-Specific vs. Shared Configuration

### Shared Hosts (Service Domains)
Examples: foo.zone, irregular.ninja, f3s.buetow.org

- Same content/routing on both servers
- Both servers have TLS certs
- Include in `@acme_hosts` without guards
- Create with prefix loops for www/standby variants

### Server-Specific Hosts (Server FQDNs)
Examples: blowfish.buetow.org, fishfinger.buetow.org

- Different per server
- Each server has ONLY its own cert
- Include in `@acme_hosts` for routing
- **Must skip in template loops**
- Use dedicated server blocks and keypair lines

### Pattern for Adding Server FQDNs

1. **Routing**: Add to `@acme_hosts` (relayd needs routing rules)
2. **ACME loop**: Skip with `next if $host eq "$hostname.$domain"`
3. **TLS loop**: Skip with `next if $host eq 'blowfish.buetow.org' or $host eq 'fishfinger.buetow.org'`
4. **Server blocks**: Use existing dedicated "Current server's FQDN" block

## Deployment Process

```bash
rex httpd relayd  # Deploy to both servers
```

Process:
1. Rex connects to both blowfish and fishfinger in parallel
2. For each server, processes templates with server-specific `$hostname`
3. Generates `/etc/httpd.conf` and `/etc/relayd.conf`
4. Writes files and restarts services via `on_change` handlers
5. Each server gets identical config structure but different hostname values

## Monitoring System (Gogios)

- Runs as user `_gogios`
- Config: `/etc/gogios.json`
- Output: `/var/www/htdocs/buetow.org/self/gogios/index.html`
- Cron schedule: Every 5 minutes between 08:00-22:00
- Check intervals: Independent from cron (e.g., TLS checks every 3600s)

**Important**: Check intervals (`RunInterval`) are independent from cron schedule. A check with 3600s interval won't re-run just because cron triggered, it runs only when interval expires.

## Configuration Testing

Before deploying:
```bash
ssh rex@server "doas httpd -n"   # Test httpd config syntax
ssh rex@server "doas relayd -n"  # Test relayd config syntax
```

After deploying:
```bash
ssh rex@server "doas rcctl check httpd"
ssh rex@server "doas rcctl check relayd"
```
