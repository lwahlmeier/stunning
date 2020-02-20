FROM alpine:3.9

RUN apk update && \
    apk -Uuv add dumb-init ca-certificates bash && \
    rm /var/cache/apk/*
COPY run.sh /run.sh
RUN touch env.sh && chmod 755 /run.sh
COPY build/stunning /stunning

COPY client/build/sclient-linux-amd64 /sclient

EXPOSE 80/tcp
EXPOSE 3478/udp

#HEALTHCHECK --interval=10s --timeout=5s --start-period=5s \
#  CMD /sclient --addr 127.0.0.1:3478


ENTRYPOINT ["/run.sh"]
CMD ["/stunning"]

