## PromDump

PromDump dumps metrics using the Prometheus endpoint instead of extracting the TSDB data blocks directly. This enables a more controlled way to extract the metrics, including filtering metrics by labels and timestamp. 

## Usage: Export Metrics

### 1. Install promdump
(Option 1) Download the binary
```shell
curl -L https://wavekit-release.s3.ap-southeast-1.amazonaws.com/promdump/download.sh | sh
```

(Option 2) Or, install it by go.
```shell
GOBIN=$(pwd) go install github.com/risingwavelabs/promdump/cmd/promdump@latest
```

### 2. Dump all metrics to a file

```shell
./promdump dump -e http://localhost:9500 --gizp
```
This will dump **ALL metrics** to a single file. Note that you can also specify the time range, and filters.  For example, to get time series with label `namespace="risingwave"`:

```shell
./promdump dump -e http://localhost:9500 --query '{namespace="risingwave"}' --gizp
```

Or use `--grafana-dashboard <file path or version>` to retrieve all metrics names needed from a Grafana dashboard, then use those metrics names to dump metrics:

```shell
./promdump dump -e http://localhost:9500 --grafana-dashboard v2.6.2
```

If you don't have internet access, you can also download the dashboard in the [offical Github repository](https://github.com/risingwavelabs/risingwave/blob/main/grafana/risingwave-user-dashboard.json) then run:

```shell
promdump dump -e http://localhost:9500 --grafana-dashboard /path/to/risingwave-user-dashboard.json
``` 

> Note: The `--query` option is not supported by Google Cloud Managed Prometheus, please check [Promdump for Google Cloud Managed Prometheus](#promdump-for-google-cloud-managed-prometheus) for more details.

### 3. Dump all metrics to multiple files
```shell
./promdump dump -e http://localhost:9500 --gzip --parts 10 --start 2025-04-20T16:40:04+08:00 --end 2025-04-22T16:40:04+08:00 --step 15s -o my-metrics
```

The `--parts` option specifies the number of files to dump to, this enable resume from the last file if the dump is interrupted. 

Note that you should use different output directory for different dump jobs, as the resume function is based on the index in the file name.

More usage can be found in `promdump -h`.

## Usage: Import Metrics to Grafana Dashboard

Setup working dir
```shell
curl https://raw.githubusercontent.com/risingwavelabs/promdump/refs/heads/main/examples/prompush/download.sh | sh
cd prompush
docker compose up -d
```

Download prompush
```shell
curl -L https://wavekit-release.s3.ap-southeast-1.amazonaws.com/prompush/download.sh | sh 
```

Push metrics to metrics store
```
./prompush -p <directory or file> -e http://localhost:8428
```

Then open [http://localhost:3001](http://localhost:3001)

## Mechanism

`promdump` simply queries the Prometheus instance to get the metrics, then streaming the result to `out.ndjson.gz`. 

`prompush` import the metrics to the VictoriMetrics.

## Troubleshooting

### Dump metrics for AWS Managed Prometheus (AMP)
As AMP doesn't support direct API access (need complex authentication with AK/SK involved), Promdump currectly does not support to connect the AMP Prometheus-compatible APIs directly. 
Please follow [this documentation](https://docs.aws.amazon.com/prometheus/latest/userguide/AMP-compatible-APIs.html) to expose all RisingWave related metrics. Make sure the result type is `matrix`, and each file can only contain one query response.

If you need to list all metrics names of RisingWave, run `promdump list-metrics --grafana-dashboard <version> > metrics.txt`. For example: 

```shell
promdump list-metrics --grafana-dashboard v2.6.2 > metrics.txt
``` 

If you don't have internet access, you can also download the dashboard in the [offical Github repository](https://github.com/risingwavelabs/risingwave/blob/main/grafana/risingwave-user-dashboard.json) then run:

```shell
promdump list-metrics --grafana-dashboard /path/to/risingwave-user-dashboard.json > metrics.txt
``` 

### Promdump for Google Cloud Managed Prometheus

Google Cloud Managed Prometheus does not support the `--query` option. Please use `--grafana-dashboard <file path or version>` argument in the Promdump CLI. Promdump will parse the grafana dashboard and get all metrics names. Then use those metrics names to construt query. 

If your environment have internet access, you can put the RisingWave version like `--grafana-dashboard v2.6.2`. Promdump will automatically fetch RisingWave dashboard from the RisingWave Github repository.

### No Data in Grafana

Check if the metrics needed by dashboard variables exist. Also check if there are any error logs in the VictoriaMetrics service.

### Prometheus: query processing would load too many samples into memory in query execution
If you encounter this error, reduce memory usage by setting `--memory-ratio` to a value less than 1. For example, `--memory-ratio 0.5` will halve the memory consumption.
If the issue persists, try progressively smaller values.

For cases where even very small memory ratios don't resolve the issue, use `--parts` to divide the query results into multiple smaller chunks. This also enable resuming from the last completed part if the dump is interrupted.
