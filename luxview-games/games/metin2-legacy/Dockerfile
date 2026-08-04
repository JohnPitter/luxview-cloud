ARG LEGACY_BASE_IMAGE=metinfiles-server:latest
FROM ${LEGACY_BASE_IMAGE}

USER root
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install -y --no-install-recommends mariadb-client mariadb-server netcat-openbsd \
    && rm -rf /var/lib/apt/lists/*

COPY assets/29052339mysql.tar.gz /tmp/openmetin-mysql.tar.gz

RUN mkdir -p /opt/openmetin/legacy \
    && tar -xzf /tmp/openmetin-mysql.tar.gz -C /opt/openmetin/legacy \
      --strip-components=1 mysql/account mysql/common mysql/log mysql/player mysql/hotbackup \
    && rm /tmp/openmetin-mysql.tar.gz \
    && mkdir -p /var/log/openmetin

COPY entrypoint.sh /usr/local/bin/openmetin-entrypoint

RUN chmod 0755 /usr/local/bin/openmetin-entrypoint

EXPOSE 11000 13001 13002 13003 13004 13099
HEALTHCHECK --interval=10s --timeout=3s --start-period=90s --retries=9 \
  CMD nc -z 127.0.0.1 11000 && nc -z 127.0.0.1 13001 || exit 1

VOLUME ["/var/lib/mysql"]
ENTRYPOINT ["/usr/local/bin/openmetin-entrypoint"]
