# Upstream extension bundle still contains PostgreSQL 18.4 and pgvector 0.8.5:
# its Renovate rule watches GitHub releases, which pgvector does not publish
# (immich-app/base-images#405). Apply the latest stable PostgreSQL patch and
# pgvector release while keeping the bundle's VectorChord 1.1.1 binary.
#
# pgvector 0.8.7 is within Immich v3.3.6's accepted range (>=0.5 <1) and the
# documented VectorChord range (>=0.7 <0.9). The base image installed pgvector
# from source without a package; the pinned PGDG package overwrites those
# files with the same paths. The database's SQL extension version is raised
# separately with ALTER EXTENSION vector UPDATE (see ../../README.md).
FROM ghcr.io/immich-app/postgres:18-vectorchord1.1.1-pgvector0.8.5@sha256:a20e21841b21b9c009daf572c844ffedfaef75fce51287bbf080edf171ef0834
ARG POSTGRES_DEB_VERSION=18.6-1.pgdg12+2
ARG PGVECTOR_DEB_VERSION=0.8.7-1.pgdg12+1
ENV PG_VERSION=${POSTGRES_DEB_VERSION}
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        postgresql-18=${POSTGRES_DEB_VERSION} \
        postgresql-client-18=${POSTGRES_DEB_VERSION} \
        libpq5=${POSTGRES_DEB_VERSION} \
        postgresql-18-pgvector=${PGVECTOR_DEB_VERSION} \
    && rm -rf /var/lib/apt/lists/*
