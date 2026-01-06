# Agent Learning Notes: Debugging 404 Errors for blowfish/fishfinger URLs

## Problem Summary

URLs `https://blowfish.buetow.org/index.txt` and `https://fishfinger.buetow.org/index.txt` were returning 404 errors instead of serving the health check files.

## Root Cause

The hostnames `blowfish.buetow.org` and `fishfinger.buetow.org` were missing from the `@acme_hosts` array in the Rexfile. This caused:

1. **No explicit routing rules in relayd**: Only hosts in `@acme_hosts` get explicit routing to `<localhost>` (httpd) in `relayd.conf.tpl:45-50`
2. **Fall-through to f3s backends**: Without routing rules, requests fell through to the default f3s cluster backends
3. **404 from f3s cluster**: The k3s cluster didn't know about these server hostnames, resulting in 404 errors

## Architecture Understanding

### Request Flow
```
Internet → relayd (port 443) → routing decision → httpd (port 8080) or f3s cluster (port 80)
```

### Key Components

1. **relayd.conf.tpl**: Reverse proxy that:
   - Terminates TLS on port 443
   - Routes requests based on Host header matching
   - Has two backend pools: `<localhost>` (httpd) and `<f3s>` (k3s cluster)
   - Falls back to f3s cluster when no explicit routing match

2. **httpd.conf.tpl**: OpenBSD httpd that:
   - Listens on port 8080 (behind relayd)
   - Serves static content for various domains
   - Has a dedicated "Current server's FQDN" block for each server's own hostname

3. **Rexfile**: Configuration management using Rex (Perl):
   - Defines `@acme_hosts` array controlling which hosts get ACME certs and routing rules
   - Templates use this array to generate both httpd and relayd configs
   - Deployed to both blowfish and fishfinger servers

## Solution Implementation

### 1. Added Hostnames to @acme_hosts (Rexfile:86)
```perl
our @acme_hosts =
  qw/.../gogios.buetow.org blowfish.buetow.org fishfinger.buetow.org/;
```

This ensures both servers are included in routing rules.

### 2. Prevented Duplicate Server Blocks (httpd.conf.tpl:3-5)
```perl
<% for my $host (@$acme_hosts) {
     # Skip current server's hostname - handled by dedicated block below
     next if $host eq "$hostname.$domain";
```

Each server has a dedicated block at lines 18-37 serving from `/htdocs/buetow.org/self`. Without this skip, adding them to `@acme_hosts` would create duplicate server blocks on port 80, causing httpd to fail.

### 3. Prevented Missing TLS Certificates (relayd.conf.tpl:25-27)
```perl
<% for my $host (@$acme_hosts) {
     # Skip server hostnames - each server only has its own cert
     next if $host eq 'blowfish.buetow.org' or $host eq 'fishfinger.buetow.org';
```

**Critical insight**: When deploying to blowfish, the config tries to load TLS certs for ALL hosts in `@acme_hosts`. But blowfish only has `blowfish.buetow.org.crt`, not `fishfinger.buetow.org.crt`. Similarly, fishfinger only has its own cert. The dedicated line `tls keypair <%= $hostname.'.'.$domain -%>` at line 31 loads the correct cert for each server.

## Debugging Methodology

### 1. Test Actual Endpoints First
```bash
curl -s https://blowfish.buetow.org/index.txt  # Test reality
```
vs. relying on monitoring dashboards which may show cached/stale data.

### 2. Check Configuration Syntax Before Deploy
```bash
ssh rex@server "doas httpd -n"   # Test httpd config
ssh rex@server "doas relayd -n"  # Test relayd config
```

### 3. Understand Monitoring Intervals
Gogios TLS checks have `RunInterval: 3600` (1 hour). After fixing issues, old failures may persist until:
- Next scheduled check
- Manual force run: `gogios -cfg /etc/gogios.json -force`

However, `-force` only updates the report timestamp, it doesn't override individual check intervals. True verification requires manual testing or waiting for interval expiry.

## Template Architecture Insights

### Variable Scoping
- `$hostname`: Current server being deployed to (blowfish or fishfinger)
- `$domain`: Domain suffix (buetow.org)
- `@acme_hosts`: Global list of all hosts needing ACME certs and routing
- `@f3s_hosts`: Hosts served by f3s k3s cluster
- `@prefixes`: ('', 'www.', 'standby.') for creating hostname variants

### Template Processing
Rex processes `.tpl` files using embedded Perl:
- `<% ... -%>`: Perl code (suppress trailing newline with -)
- `<%= $var %>`: Print variable
- Templates are processed per-server with different `$hostname` values

### Common Pitfall: Server-Specific vs. Shared Configuration
When adding a hostname to a shared array like `@acme_hosts`, consider:
1. Does each server have the required TLS certificates?
2. Will this create duplicate server blocks?
3. Is this hostname server-specific (like server FQDNs) or shared (like service domains)?

For server FQDNs (blowfish.buetow.org, fishfinger.buetow.org):
- **Routing**: Needs to be in `@acme_hosts` for relayd routing rules
- **Server blocks**: Skip in loops, use dedicated blocks instead
- **TLS certs**: Skip in loops, use dedicated keypair line instead

## Key Learnings

1. **Routing is explicit, not implicit**: Just because httpd has a server block doesn't mean relayd will route to it. Routing rules must be configured separately.

2. **Certificate management per server**: In a multi-server setup, each server only has its own certificate, not certificates for other servers in the pool.

3. **Template loops need guards**: When iterating over shared arrays in templates that deploy to multiple servers, check if items need server-specific handling.

4. **Monitoring vs. reality**: Always verify fixes by testing actual endpoints. Monitoring systems may show stale data due to caching intervals.

5. **Configuration deployment is atomic**: Rex deploys templates and restarts services. Brief service interruptions during restarts can trigger monitoring alerts that resolve once services stabilize.

## Files Modified

1. `Rexfile` - Added blowfish/fishfinger to @acme_hosts
2. `etc/httpd.conf.tpl` - Skip current hostname in @acme_hosts loop
3. `etc/relayd.conf.tpl` - Skip server hostnames in TLS keypair loop

## Deployment Process

```bash
rex httpd relayd  # Deploy to both servers in parallel
```

Rex connects to both blowfish and fishfinger, generates configs with server-specific `$hostname` values, deploys files, and restarts services.

## Verification

```bash
# Test endpoints
curl -s https://blowfish.buetow.org/index.txt   # Should return health check text
curl -s https://fishfinger.buetow.org/index.txt # Should return health check text

# Check service status
ssh -p 2 rex@server "doas rcctl check httpd && doas rcctl check relayd"
```

## Future Considerations

When adding new server hostnames:
1. Add to `@acme_hosts` for routing
2. Add `next if $host eq "$hostname.$domain"` guards in template loops
3. Ensure dedicated blocks exist for server-specific config
4. Remember each server only has its own TLS certificate
5. Test config syntax before deploying
6. Verify endpoints after deployment, don't rely on monitoring
