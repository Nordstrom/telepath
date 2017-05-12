FROM scratch

COPY bin/release/telepath /telepath
EXPOSE 8089
ENTRYPOINT ["/telepath"]
