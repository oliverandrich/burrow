# syntax=docker/dockerfile:1
# SPDX-License-Identifier: MIT
#
# Built and pushed by goreleaser's dockers_v2 step. Binaries are
# pre-built by `goreleaser build`; this Dockerfile just packages them.
# For multi-arch builds buildx lays the binaries out per platform under
# ${TARGETPLATFORM}/ (e.g. linux/amd64/__ProjectName__).

FROM gcr.io/distroless/static-debian12:nonroot

ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/__ProjectName__ /usr/local/bin/__ProjectName__

# Burrow's default listen port. Override at run time with --port.
EXPOSE 8080

# SQLite lives at /data/app.db by default. Mount a host directory or a
# named volume here so the database survives container restarts:
#
#   docker run -v $PWD/data:/data -p 8080:8080 \
#     -e DATABASE_DSN=sqlite:////data/app.db \
#     ghcr.io/__GitUser__/__ProjectName__:latest
#
# The mounted directory must be writable by UID 65532 (the distroless
# nonroot user). For a fresh host dir:
#   mkdir -p data && sudo chown 65532:65532 data

ENTRYPOINT ["/usr/local/bin/__ProjectName__"]
