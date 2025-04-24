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
./promdump -e http://localhost:9500 --gizp
```
This will dump **ALL metrics** to a single file. Note that you can also specify the time range, and filters.  For example, to get time series with label `namespace="risingwave"`:
```shell
./promdump -e http://localhost:9500 --query '{namespace="risingwave"}' --gizp
```

### 3. Dump all metrics to multiple files
```shell
./promdump -e http://localhost:9500 --gzip --parts 10 --start 2025-04-20T16:40:04+08:00 --end 2025-04-22T16:40:04+08:00 --step 15s -o my-metrics
```

The `--parts` option specifies the number of files to dump to, this enable resume from the last file if the dump is interrupted. 

Note that you should use different output directory for different dump jobs, as the resume function is based on the index in the file name.

More usage can be found in `promdump -h`.

## Usage: Import Metrics to Grafana Dashboard

Setup working dir
```shell
curl https://raw.githubusercontent.com/risingwavelabs/promdump/refs/heads/main/examples/prompush/download.sh | sh
cd prompush
docker-compose up -d
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

