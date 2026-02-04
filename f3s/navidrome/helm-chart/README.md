# Navidrome Music Server Kubernetes Deployment

This directory contains the Kubernetes configuration for deploying Navidrome, a modern music server and streamer compatible with Subsonic/Airsonic clients.

## Overview

- **Application**: Navidrome
- **Image**: `deluan/navidrome:latest`
- **Namespace**: `services`
- **Ingress**: `navidrome.f3s.buetow.org`
- **Port**: 4533

## Storage

Navidrome requires two persistent volumes:

1. **Data Volume** (`/data/nfs/k3svolumes/navidrome/data`): 10Gi
   - Stores configuration, database (SQLite), cache, and metadata
   - Mounted at `/data` in the container

2. **Music Volume** (`/data/nfs/k3svolumes/navidrome/music`): 200Gi
   - Stores your music library
   - Mounted at `/music` in the container

## Pre-Deployment Setup

Before deploying, ensure the storage directories exist on the host:

```bash
mkdir -p /data/nfs/k3svolumes/navidrome/data
mkdir -p /data/nfs/k3svolumes/navidrome/music
```

## Deployment

The application is managed by ArgoCD. After committing changes to git and pushing to the r0 remote, ArgoCD will automatically sync and deploy the application.

To manually trigger a sync:

```bash
just sync
```

## Initial Configuration

1. After deployment, access Navidrome at `http://navidrome.f3s.buetow.org`
2. On first access, you'll be prompted to create an admin account
3. Configure settings as needed through the web UI

## Adding Music

Copy or sync your music files to the music volume on the host:

```bash
# On the host with the NFS share
cp -r /path/to/your/music/* /data/nfs/k3svolumes/navidrome/music/
```

Navidrome will automatically scan for new music based on the `ND_SCANSCHEDULE` setting (default: hourly).

## Management Commands

Check deployment status:

```bash
just status
```

View logs:

```bash
just logs
```

Port forward for local access:

```bash
just port-forward
```

Restart the application:

```bash
just restart
```

## Features

- Web-based music player
- Compatible with Subsonic/Airsonic mobile apps
- Automatic metadata fetching from Last.fm
- Playlist support
- Multi-user support
- Transcoding support
- Scrobbling to Last.fm

## Client Apps

Navidrome is compatible with various Subsonic clients:

- **iOS**: play:Sub, substreamer
- **Android**: DSub, Ultrasonic, substreamer
- **Desktop**: Sublime Music, Sonixd
- **Web**: Built-in web player

## Configuration

Environment variables can be adjusted in the deployment.yaml:

- `ND_SCANSCHEDULE`: How often to scan for new music (default: 1h)
- `ND_LOGLEVEL`: Logging level (info, debug, trace)
- `ND_BASEURL`: Base URL if behind a reverse proxy

See [Navidrome documentation](https://www.navidrome.org/docs/usage/configuration-options/) for all available options.

## Troubleshooting

Check pod status:

```bash
kubectl get pods -n services | grep navidrome
```

View detailed logs:

```bash
kubectl logs -n services -l app=navidrome --tail=200
```

Check persistent volume claims:

```bash
kubectl get pvc -n services | grep navidrome
```

## Security

- Authentication is handled by Navidrome itself
- No secrets are stored in git
- User accounts are managed through the web UI
- Consider enabling HTTPS in production
