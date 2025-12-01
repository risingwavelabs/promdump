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

### Promdump for Google Cloud Managed Prometheus

Google Cloud Managed Prometheus does not support the `--query` option. Instead, you must specify metric names in a file.

Promdump have embeded some metrics name files, add `--use-preset-metrics-names default`, this will import [the metrics names preset here](https://github.com/risingwavelabs/promdump/tree/main/static). For example:

```shell
promdump dump -e <your GMP endpoint> --use-preset-metrics-names default
```

If these preset files does not contain the metrics name you need, please follow the following instruction to fetch the metrics names file of your RisingWave cluster:

1. To obtain all metrics names in RisingWave, run a local RisingWave instance using the following command (replace `latest` with your desired version):

    ```shell
    docker run --rm -p 4566:4566 -p 1250:1250 --entrypoint /risingwave/bin/risingwave risingwavelabs/risingwave:latest standalone --meta-opts="--listen-addr 0.0.0.0:5690 --advertise-addr localhost:5690 --dashboard-host 0.0.0.0:5691 --prometheus-host 0.0.0.0:1250 --backend sqlite  --sql-endpoint /root/single_node.db --state-store hummock+fs:///root/state_store --data-directory hummock_001" --compute-opts="--listen-addr 0.0.0.0:5688 --prometheus-listener-addr 0.0.0.0:1250 --advertise-addr localhost:5688 --async-stack-trace verbose --parallelism 4 --total-memory-bytes 2147483648 --role both --meta-address http://0.0.0.0:5690" --frontend-opts="--listen-addr 0.0.0.0:4566 --advertise-addr localhost:4566 --prometheus-listener-addr 0.0.0.0:1250 --health-check-listener-addr 0.0.0.0:6786 --meta-addr http://0.0.0.0:5690 --frontend-total-memory-bytes=500000000" --compactor-opts=" --listen-addr 0.0.0.0:6660 --prometheus-listener-addr 0.0.0.0:1250 --advertise-addr localhost:6660 --meta-address http://0.0.0.0:5690 --compactor-total-memory-bytes=1000000000"
    ```

    The default standalone mode doesn't expose Prometheus metrics, so we need to explicitly configure all components with their Prometheus listener addresses.

2. Get all metrics names from the RisingWave instance you just run:
    ```shell
    promdump list-metrics --exporter http://localhost:1250 > metrics.txt
    ```
    This will generate a file `metrics.txt` containing all metric names. You can now stop the RisingWave instance.

3. Run PromDump:
    ```shell
    promdump dump -e <your GMP endpoint> --metrics-names metrics.txt --gzip
    ```

### No Data in Grafana

Check the Docker Compose logs for any errors in the VictoriaMetrics container. If there are errors related to vmimport, try using the `--use-legacy-format` flag with prompush.


### Prometheus: query processing would load too many samples into memory in query execution
If you encounter this error, reduce memory usage by setting `--memory-ratio` to a value less than 1. For example, `--memory-ratio 0.5` will halve the memory consumption.
If the issue persists, try progressively smaller values.

For cases where even very small memory ratios don't resolve the issue, use `--parts` to divide the query results into multiple smaller chunks. This also enable resuming from the last completed part if the dump is interrupted.
