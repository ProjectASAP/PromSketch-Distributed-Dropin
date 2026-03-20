from prometheus_client.core import GaugeMetricFamily, REGISTRY
from prometheus_client import start_http_server
from prometheus_client.registry import Collector
from prometheus_client import PROCESS_COLLECTOR, PLATFORM_COLLECTOR, GC_COLLECTOR
import argparse
import sys
import time
import numpy


class CustomCollector(Collector):

    def __init__(self, num_machines, machine_id_start):
        self.num_machines = num_machines
        self.machine_id_start = machine_id_start

    def collect(self):
        fake_metric = GaugeMetricFamily(
            "fake_machine_metric",
            "Generating fake machine time series data with Zipf distribution",
            labels=["machineid"],
        )
        for i in range(self.machine_id_start, self.machine_id_start + self.num_machines):
            value = -1
            while value < 0 or value > 100000:
                value = numpy.random.zipf(1.01)
            fake_metric.add_metric([f"machine_{i}"], value=value)

        yield fake_metric


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Fake Zipf metric exporter")
    parser.add_argument("--port", type=int, default=8080)
    parser.add_argument("--instancestart", type=int, default=0)
    parser.add_argument("--batchsize", type=int, default=1000)
    args = parser.parse_args()

    REGISTRY.unregister(PROCESS_COLLECTOR)
    REGISTRY.unregister(PLATFORM_COLLECTOR)
    REGISTRY.unregister(GC_COLLECTOR)
    REGISTRY.register(CustomCollector(args.batchsize, args.instancestart))
    start_http_server(port=args.port)
    while True:
        time.sleep(1)
