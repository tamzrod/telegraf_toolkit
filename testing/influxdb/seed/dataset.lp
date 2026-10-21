# Synthetic line-protocol dataset for the isolated InfluxDB integration tests.
# NON-PRODUCTION test data only. Deterministic timestamps (UTC nanoseconds) so
# write/query results are reproducible across runs.
#
# Schema under test:
#   measurement: environment
#     tags:   host, region
#     fields: used_percent (float), temp_c (float), up (integer), note (string)
#   measurement: sensor
#     tags:   host, sensor_id
#     fields: value (float)
#
# Timestamps are 2023-11-14T22:13:20Z + n*60s (1700000000000000000 ns base).

environment,host=host-a,region=eu-west used_percent=41.5,temp_c=21.0,up=1i,note="nominal" 1700000000000000000
environment,host=host-a,region=eu-west used_percent=43.5,temp_c=22.5,up=1i,note="nominal" 1700000060000000000
environment,host=host-b,region=eu-west used_percent=55.0,temp_c=24.0,up=1i,note="warm" 1700000000000000000
environment,host=host-b,region=eu-west used_percent=58.25,temp_c=25.5,up=0i,note="warm" 1700000060000000000
environment,host=host-c,region=us-east used_percent=12.0,temp_c=18.0,up=1i,note="cold" 1700000000000000000
sensor,host=host-a,sensor_id=s1 value=3.14 1700000000000000000
sensor,host=host-a,sensor_id=s1 value=3.15 1700000060000000000
sensor,host=host-b,sensor_id=s2 value=2.71 1700000000000000000
