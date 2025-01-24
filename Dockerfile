FROM scratch
ADD epdtrh_exporter /epdtrh_exporter
ENTRYPOINT ["/epdtrh_exporter"]
