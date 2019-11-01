#!/usr/bin/env python2
# -*- coding: utf-8 -*-

import yaml
import time
from prometheus_client import start_http_server, Gauge, CollectorRegistry

# Create a metric to track time spent and requests made.
master_log_pos = Gauge('lightning_master_log_pos', 'replication master log position', ['server_id', 'port'])
seconds_behind_master = Gauge('lightning_seconds_behind_master', 'seconds behind master', ['server_id', 'port'])

def snap_metrics():
    with open('master.info') as f:
        metrics = yaml.load(f, Loader=yaml.Loader)
    master_log_pos.labels(metrics["server-id"], metrics["master_port"]).set(metrics["master_log_pos"])
    seconds_behind_master.labels(metrics["server-id"], metrics["master_port"]).set(metrics["seconds_behind_master"])

if __name__ == '__main__':
    # Start up the server to expose the metrics.
    start_http_server(8100, '0.0.0.0')
    # Generate some requests.
    while True:
        snap_metrics()
        time.sleep(10)
