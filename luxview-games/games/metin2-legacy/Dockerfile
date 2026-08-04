FROM i386/debian:bullseye-slim AS builder

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
      build-essential file g++ libboost-dev libdevil-dev libgtest-dev \
      libjpeg-dev liblua5.1-0-dev libmariadb-dev libmariadb-dev-compat \
      libpng-dev libssl-dev libtiff-dev make patch pkg-config zlib1g-dev \
    && rm -rf /var/lib/apt/lists/*

COPY assets/05062349source.tar.gz /tmp/openmetin-source.tar.gz

RUN mkdir -p /opt \
    && tar -xzf /tmp/openmetin-source.tar.gz -C /opt \
    && rm /tmp/openmetin-source.tar.gz \
    && touch /opt/aSource/Extern/cryptopp/pch.h \
    && mv /opt/aSource/Extern/include/boost /opt/aSource/Extern/include/boost.legacy \
    && sed -i 's#^CFLAGS = #CFLAGS = -include /opt/openmetin-compat.h #' /opt/aSource/Server/game/src/Makefile \
    && sed -i 's#^CFLAGS = #CFLAGS = -include /opt/openmetin-compat.h #' /opt/aSource/Server/db/src/Makefile \
    && mkdir -p /usr/game/share/bin

RUN make -C /opt/aSource/Extern/cryptopp \
      CXXFLAGS="-DNDEBUG -g -O2 -fpermissive -Wno-narrowing -I../include/cryptopp -I."

RUN apt-get update \
    && apt-get install -y --no-install-recommends libmd-dev libmng-dev \
    && rm -rf /var/lib/apt/lists/*

COPY assets/build/compat.h /opt/openmetin-compat.h
COPY assets/build/vid-hash.patch /tmp/vid-hash.patch
COPY assets/build/client-dirent.patch /tmp/client-dirent.patch
COPY assets/build/libsql-mariadb.patch /tmp/libsql-mariadb.patch
COPY assets/build/combat-zone-return.patch /tmp/combat-zone-return.patch
COPY assets/build/bind-all-interfaces.patch /tmp/bind-all-interfaces.patch
COPY assets/build/FileMonitor_FreeBSD.h /opt/aSource/Server/game/src/FileMonitor_FreeBSD.h
COPY assets/build/FileMonitor_FreeBSD.cpp /opt/aSource/Server/game/src/FileMonitor_FreeBSD.cpp

RUN cd /opt/aSource/Server/game/src \
    && patch < /tmp/vid-hash.patch \
    && patch < /tmp/client-dirent.patch \
    && patch < /tmp/combat-zone-return.patch \
    && patch < /tmp/bind-all-interfaces.patch \
    && cd /opt/aSource/Server/libsql \
    && patch < /tmp/libsql-mariadb.patch \
    && rm /tmp/vid-hash.patch /tmp/client-dirent.patch /tmp/combat-zone-return.patch /tmp/bind-all-interfaces.patch /tmp/libsql-mariadb.patch \
    && cd /opt/aSource/Server/game/src \
    && sed -i 's#^LIBDIR =$#LIBDIR = -L/usr/lib/i386-linux-gnu#' Makefile /opt/aSource/Server/db/src/Makefile \
    && sed -i 's#-O3#-O2 -fno-strict-aliasing -fwrapv -fno-omit-frame-pointer#' Makefile \
    && sed -i 's#-llcms#-llcms2#g' Makefile \
    && sed -i 's#^CFLAGS = #CFLAGS = -include /usr/include/signal.h -include /opt/openmetin-compat.h #' /opt/aSource/Server/libthecore/src/Makefile \
    && sed -i 's#^CFLAGS = #CFLAGS = -include /opt/openmetin-compat.h #' /opt/aSource/Server/libsql/Makefile \
    && sed -i 's#^IFLAGS =.*#IFLAGS = -I/usr/include/mariadb#' /opt/aSource/Server/libsql/Makefile \
    && sed -i 's|^#ifndef __WIN32__$|#if !defined(__WIN32__) \&\& !defined(__USE_SELECT__)|' /opt/aSource/Server/libthecore/include/fdwatch.h \
    && sed -i 's#select(0,#select(FD_SETSIZE,#g' /opt/aSource/Server/libthecore/src/fdwatch.c \
    && sed -i 's|^#elif __FreeBSD__$|#else|' /opt/aSource/Server/libthecore/src/signal.c

WORKDIR /opt/aSource/Server

RUN make -C liblua clean CC=gcc && make -C liblua CC=gcc
RUN cd libthecore/src && make clean CC=g++ && touch Depend && make CC=g++
RUN cd libpoly && make clean CXX=g++ && touch Depend && make CXX=g++
RUN cd libgame/src && make clean CXX=g++ && touch Depend && make CXX=g++
RUN cd libsql && make clean CXX=g++ && touch Depend && make CXX=g++

RUN rm -f /opt/aSource/Server/game/src/Depend /opt/aSource/Server/db/src/Depend \
    && rm -f /opt/aSource/Server/game/src/OBJDIR/combat_zone.o /opt/aSource/Server/game/src/OBJDIR/main.o /opt/aSource/Server/game/src/OBJDIR/minilzo.o \
    && rm -f /opt/aSource/Server/db/src/OBJDIR_DB/* \
    && make -C /opt/aSource/Server/game/src CC=g++ CXX=g++ \
    && make -C /opt/aSource/Server/db/src CC=g++ CXX=g++

FROM i386/debian:bullseye-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
      bash libdevil1c2 libjpeg62-turbo liblcms2-2 libmariadb3 libmd0 \
      libmng1 libpng16-16 libssl1.1 libstdc++6 libtiff5 mariadb-client \
      mariadb-server netcat-openbsd zlib1g \
    && rm -rf /var/lib/apt/lists/*

COPY assets/05062349game.tar.gz /tmp/openmetin-game.tar.gz
COPY assets/29052339mysql.tar.gz /tmp/openmetin-mysql.tar.gz

RUN tar -xzf /tmp/openmetin-game.tar.gz -C /usr \
    && rm /tmp/openmetin-game.tar.gz \
    && rm -f /usr/game/share/bin/game /usr/game/share/bin/db \
    && mkdir -p /opt/openmetin/legacy \
    && tar -xzf /tmp/openmetin-mysql.tar.gz -C /opt/openmetin/legacy \
      --strip-components=1 mysql/account mysql/common mysql/log mysql/player mysql/hotbackup \
    && rm /tmp/openmetin-mysql.tar.gz

COPY assets/localization/server-pt-BR/game/share/ /usr/game/share/

RUN find /usr/game/core -type d -path '*/locale/turkey' -exec sh -c \
    'cp -a /usr/game/share/locale/turkey/. "$1/"' _ {} \; \
    && mkdir -p /var/log/openmetin

COPY --from=builder /usr/game/share/bin/game /usr/game/share/bin/game
COPY --from=builder /opt/aSource/Server/db/db_r42067 /usr/game/share/bin/db
COPY entrypoint.sh /usr/local/bin/openmetin-entrypoint

RUN chmod 0755 /usr/game/share/bin/game /usr/game/share/bin/db /usr/local/bin/openmetin-entrypoint

EXPOSE 11000 13001 13002 13003 13004 13099
HEALTHCHECK --interval=10s --timeout=3s --start-period=90s --retries=9 \
  CMD nc -z 127.0.0.1 11000 && nc -z 127.0.0.1 13001 || exit 1

VOLUME ["/var/lib/mysql"]
ENTRYPOINT ["/usr/local/bin/openmetin-entrypoint"]
