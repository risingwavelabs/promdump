import subprocess
import time
import urllib.request
import urllib.error
import json
import os

def _wait_for_http_ready(url: str, timeout: float = 3600.0, interval: float = 0.5) -> None:
    deadline = time.time() + timeout
    last_exc = None
    while time.time() < deadline:
        try:
            with urllib.request.urlopen(url, timeout=5) as resp:
                if 200 <= getattr(resp, "status", 0) < 300:
                    return
        except (urllib.error.URLError, urllib.error.HTTPError, ConnectionError) as e:
            last_exc = e
        time.sleep(interval)
    raise TimeoutError(f"Timed out waiting for {url}") from last_exc


def export(version: str):
    filename = f"metrics/{version}.txt"
    if os.path.exists(filename):
        print(f"Metrics for version {version} already exported. Skipping.")
        return

    subprocess.run([
        "docker",
        "run",
        "--rm",
        "-d",
        "-p", "4566:4566",
        "-p", "1250:1250",
        "--name", "risingwave_export_metrics",
        "--entrypoint", "/risingwave/bin/risingwave",
        f"risingwavelabs/risingwave:{version}",
        "standalone",
        "--meta-opts=--listen-addr 0.0.0.0:5690 --advertise-addr localhost:5690 --dashboard-host 0.0.0.0:5691 --prometheus-host 0.0.0.0:1250 --backend sqlite  --sql-endpoint /root/single_node.db --state-store hummock+fs:///root/state_store --data-directory hummock_001",
        "--compute-opts=--listen-addr 0.0.0.0:5688 --prometheus-listener-addr 0.0.0.0:1250 --advertise-addr localhost:5688 --async-stack-trace verbose --parallelism 4 --total-memory-bytes 2147483648 --role both --meta-address http://0.0.0.0:5690",
        "--frontend-opts=--listen-addr 0.0.0.0:4566 --advertise-addr localhost:4566 --prometheus-listener-addr 0.0.0.0:1250 --health-check-listener-addr 0.0.0.0:6786 --meta-addr http://0.0.0.0:5690 --frontend-total-memory-bytes=500000000",
        "--compactor-opts=--listen-addr 0.0.0.0:6660 --prometheus-listener-addr 0.0.0.0:1250 --advertise-addr localhost:6660 --meta-address http://0.0.0.0:5690 --compactor-total-memory-bytes=1000000000",
    ])

    try:
        _wait_for_http_ready("http://localhost:1250")

        with open(filename, "w") as f:
            subprocess.run([
                "promdump",
                "list-metrics",
                "--exporter",
                "http://localhost:1250",
            ], stdout=f, check=True)
    except Exception as e:
        print(f"Failed to export metrics for version {version}: {e}")
    finally:
        print("Shutting down RisingWave container...")
        subprocess.run(
            ["docker", "stop", "risingwave_export_metrics"],
            check=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
        )

        time.sleep(5)

        result = subprocess.run(
            ["docker", "image", "rm", f"risingwavelabs/risingwave:{version}"],
            check=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
        )
        for line in result.stdout.splitlines():
            print(line)


def main():
    url = "https://api.github.com/repos/risingwavelabs/risingwave/tags?per_page=30"

    with urllib.request.urlopen(url) as resp:
        tags = json.load(resp)
        for tag in tags:
            version = tag["name"]
            print(f"Exporting metrics for version: {version}")
            export(version)

if __name__ == "__main__":
    main()