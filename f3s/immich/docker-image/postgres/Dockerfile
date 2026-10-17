# Upstream extension bundle still contains PostgreSQL 18.4. Apply the latest
# stable PostgreSQL patch while retaining its supported vector extensions.
FROM ghcr.io/immich-app/postgres:18-vectorchord1.1.1-pgvector0.8.5@sha256:a20e21841b21b9c009daf572c844ffedfaef75fce51287bbf080edf171ef0834
ARG POSTGRES_DEB_VERSION=18.6-1.pgdg12+2
ENV PG_VERSION=${POSTGRES_DEB_VERSION}
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        postgresql-18=${POSTGRES_DEB_VERSION} \
        postgresql-client-18=${POSTGRES_DEB_VERSION} \
        libpq5=${POSTGRES_DEB_VERSION} \
    && rm -rf /var/lib/apt/lists/*
